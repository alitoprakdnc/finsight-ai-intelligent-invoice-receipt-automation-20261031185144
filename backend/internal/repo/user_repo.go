package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"finSight/internal/model"
)

// UserRepo provides CRUD operations for users, password handling and token revocation.
type UserRepo struct {
	db    *sqlx.DB
	redis *redis.Client
}

// NewUserRepo creates a new UserRepo instance.
func NewUserRepo(db *sqlx.DB, redisClient *redis.Client) *UserRepo {
	return &UserRepo{
		db:    db,
		redis: redisClient,
	}
}

// CreateUser creates a new user with a hashed password.
// It returns the created user with its generated ID.
func (r *UserRepo) CreateUser(ctx context.Context, u *model.User, plainPassword string) (*model.User, error) {
	if u == nil {
		return nil, errors.New("user is nil")
	}
	if plainPassword == "" {
		return nil, errors.New("password cannot be empty")
	}

	hashed, err := hashPassword(plainPassword)
	if err != nil {
		return nil, errors.Wrap(err, "hashing password")
	}
	u.PasswordHash = hashed

	query := `
		INSERT INTO users (email, password_hash, role, preferences, created_at, updated_at)
		VALUES (:email, :password_hash, :role, :preferences, NOW(), NOW())
		RETURNING id, created_at, updated_at
	`
	// sqlx.NamedExec does not support RETURNING, so we use NamedQuery.
	rows, err := r.db.NamedQueryContext(ctx, query, u)
	if err != nil {
		return nil, errors.Wrap(err, "inserting user")
	}
	defer rows.Close()

	if rows.Next() {
		if err := rows.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, errors.Wrap(err, "scanning inserted user")
		}
	} else {
		return nil, errors.New("no user row returned after insert")
	}
	return u, nil
}

// GetUserByID fetches a user by its primary key.
func (r *UserRepo) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	const query = `
		SELECT id, email, password_hash, role, preferences, created_at, updated_at
		FROM users
		WHERE id = $1
	`
	var u model.User
	if err := r.db.GetContext(ctx, &u, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // not found
		}
		return nil, errors.Wrap(err, "querying user by id")
	}
	return &u, nil
}

// GetUserByEmail fetches a user by its email address.
func (r *UserRepo) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	const query = `
		SELECT id, email, password_hash, role, preferences, created_at, updated_at
		FROM users
		WHERE email = $1
	`
	var u model.User
	if err := r.db.GetContext(ctx, &u, query, email); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // not found
		}
		return nil, errors.Wrap(err, "querying user by email")
	}
	return &u, nil
}

// UpdateUser updates mutable fields of a user (email, role, preferences).
// Password must be changed via SetPassword.
func (r *UserRepo) UpdateUser(ctx context.Context, u *model.User) error {
	if u == nil {
		return errors.New("user is nil")
	}
	const query = `
		UPDATE users
		SET email = :email,
		    role = :role,
		    preferences = :preferences,
		    updated_at = NOW()
		WHERE id = :id
	`
	_, err := r.db.NamedExecContext(ctx, query, u)
	if err != nil {
		return errors.Wrap(err, "updating user")
	}
	return nil
}

// DeleteUser removes a user from the database.
// All related data (invoices, webhooks, etc.) should be cascade‑deleted via FK constraints.
func (r *UserRepo) DeleteUser(ctx context.Context, id int64) error {
	const query = `DELETE FROM users WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, "deleting user")
	}
	// Also clean up any token revocation entries for this user (optional).
	return nil
}

// SetPassword hashes the provided plain password and updates the user's password_hash.
func (r *UserRepo) SetPassword(ctx context.Context, userID int64, plainPassword string) error {
	if plainPassword == "" {
		return errors.New("password cannot be empty")
	}
	hashed, err := hashPassword(plainPassword)
	if err != nil {
		return errors.Wrap(err, "hashing password")
	}
	const query = `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`
	_, err = r.db.ExecContext(ctx, query, hashed, userID)
	if err != nil {
		return errors.Wrap(err, "updating password hash")
	}
	return nil
}

// VerifyPassword checks whether the supplied plain password matches the stored hash.
func (r *UserRepo) VerifyPassword(ctx context.Context, email, plainPassword string) (bool, error) {
	user, err := r.GetUserByEmail(ctx, email)
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil // user not found
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(plainPassword))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, errors.Wrap(err, "comparing password hash")
	}
	return true, nil
}

// ListUsers returns a paginated list of users.
// If limit is 0 a sensible default (50) is used.
func (r *UserRepo) ListUsers(ctx context.Context, offset, limit int) ([]*model.User, error) {
	if limit <= 0 {
		limit = 50
	}
	const query = `
		SELECT id, email, role, preferences, created_at, updated_at
		FROM users
		ORDER BY id ASC
		OFFSET $1 LIMIT $2
	`
	rows, err := r.db.QueryxContext(ctx, query, offset, limit)
	if err != nil {
		return nil, errors.Wrap(err, "listing users")
	}
	defer rows.Close()

	var users []*model.User
	for rows.Next() {
		var u model.User
		if err := rows.StructScan(&u); err != nil {
			return nil, errors.Wrap(err, "scanning user row")
		}
		users = append(users, &u)
	}
	return users, nil
}

// ---------------------------------------------------------------------
// Token revocation (stateless JWT) support
// ---------------------------------------------------------------------

// RevokeToken stores a JWT identifier (jti) in Redis until the token would naturally expire.
// The caller should provide the remaining TTL of the token.
func (r *UserRepo) RevokeToken(ctx context.Context, jti string, ttl time.Duration) error {
	if jti == "" {
		return errors.New("jti cannot be empty")
	}
	if ttl <= 0 {
		ttl = time.Hour // fallback safety
	}
	key := fmt.Sprintf("revoked_jti:%s", jti)
	if err := r.redis.Set(ctx, key, "revoked", ttl).Err(); err != nil {
		return errors.Wrap(err, "storing revoked token in redis")
	}
	return nil
}

// IsTokenRevoked checks whether a given jti is present in the revocation store.
func (r *UserRepo) IsTokenRevoked(ctx context.Context, jti string) (bool, error) {
	if jti == "" {
		return false, errors.New("jti cannot be empty")
	}
	key := fmt.Sprintf("revoked_jti:%s", jti)
	val, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, errors.Wrap(err, "checking revoked token")
	}
	return val == "revoked", nil
}

// ---------------------------------------------------------------------
// Helper functions
// ---------------------------------------------------------------------

func hashPassword(pw string) (string, error) {
	const cost = bcrypt.DefaultCost
	hashed, err := bcrypt.GenerateFromPassword([]byte(pw), cost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// UpdatePreferences stores a JSON‑serializable preferences struct for a user.
// The preferences type is left generic; callers marshal their struct before invoking.
func (r *UserRepo) UpdatePreferences(ctx context.Context, userID int64, prefs interface{}) error {
	data, err := json.Marshal(prefs)
	if err != nil {
		return errors.Wrap(err, "marshalling preferences")
	}
	const query = `UPDATE users SET preferences = $1, updated_at = NOW() WHERE id = $2`
	_, err = r.db.ExecContext(ctx, query, data, userID)
	if err != nil {
		return errors.Wrap(err, "updating preferences")
	}
	return nil
}

// GetPreferences unmarshals the stored JSON preferences into the provided destination.
// dst must be a pointer to the expected struct.
func (r *UserRepo) GetPreferences(ctx context.Context, userID int64, dst interface{}) error {
	const query = `SELECT preferences FROM users WHERE id = $1`
	var raw json.RawMessage
	if err := r.db.GetContext(ctx, &raw, query, userID); err != nil {
		return errors.Wrap(err, "fetching preferences")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return errors.Wrap(err, "unmarshalling preferences")
	}
	return nil
}