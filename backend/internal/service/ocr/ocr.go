package ocr

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"finSight/internal/config"
)

// Engine represents the OCR engine to be used.
type Engine string

const (
	EngineTesseract    Engine = "tesseract"
	EngineGoogleVision Engine = "google_vision"
)

// WordConfidence holds the extracted word and its confidence score.
type WordConfidence struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"` // 0‑100
}

// Result is the outcome of an OCR operation.
type Result struct {
	Text            string           `json:"text"`             // full plain text
	MeanConfidence  float64          `json:"mean_confidence"`  // average confidence across words
	WordConfidences []WordConfidence `json:"word_confidences"` // per‑word details
}

// Service provides OCR capabilities.
type Service struct {
	engine          Engine
	tesseractBinary string
	logger          *zap.Logger
	// googleVisionClient *vision.ImageAnnotatorClient // placeholder for future extension
}

// New creates a new OCR service based on the application configuration.
func New(cfg *config.Config, logger *zap.Logger) (*Service, error) {
	if cfg == nil {
		return nil, errors.New("nil config")
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	engine := Engine(strings.ToLower(strings.TrimSpace(cfg.OCR.Engine)))
	if engine == "" {
		engine = EngineTesseract
	}

	s := &Service{
		engine: engine,
		logger: logger,
	}

	// Resolve tesseract binary path; allow override via config.
	if cfg.OCR.TesseractBinary != "" {
		s.tesseractBinary = cfg.OCR.TesseractBinary
	} else {
		s.tesseractBinary = "tesseract"
	}

	// Validate binary existence (only for tesseract).
	if s.engine == EngineTesseract {
		if _, err := exec.LookPath(s.tesseractBinary); err != nil {
			return nil, errors.Wrap(err, "tesseract binary not found in PATH")
		}
	}

	// Future: initialise Google Vision client if needed.
	// if s.engine == EngineGoogleVision { ... }

	return s, nil
}

// Process runs OCR on the supplied file and returns extracted text and confidence scores.
func (s *Service) Process(ctx context.Context, filePath string) (*Result, error) {
	if filePath == "" {
		return nil, errors.New("empty file path")
	}
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to resolve absolute path")
	}

	switch s.engine {
	case EngineTesseract:
		return s.processWithTesseract(ctx, absPath)
	case EngineGoogleVision:
		// Placeholder for future implementation.
		return nil, errors.New("google vision OCR not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported OCR engine: %s", s.engine)
	}
}

// processWithTesseract runs the Tesseract binary twice: once for plain text and once for TSV to obtain confidence scores.
func (s *Service) processWithTesseract(ctx context.Context, filePath string) (*Result, error) {
	// 1️⃣ Extract plain text.
	text, err := s.runTesseractText(ctx, filePath)
	if err != nil {
		s.logger.Error("tesseract text extraction failed", zap.Error(err), zap.String("file", filePath))
		return nil, err
	}

	// 2️⃣ Extract TSV for confidence.
	wordConfs, err := s.runTesseractTSV(ctx, filePath)
	if err != nil {
		s.logger.Error("tesseract TSV extraction failed", zap.Error(err), zap.String("file", filePath))
		return nil, err
	}

	meanConf := calculateMeanConfidence(wordConfs)

	return &Result{
		Text:            text,
		MeanConfidence: meanConf,
		WordConfidences: wordConfs,
	}, nil
}

// runTesseractText executes tesseract to obtain plain text output.
func (s *Service) runTesseractText(ctx context.Context, filePath string) (string, error) {
	// tesseract <input> stdout -l eng
	cmd := exec.CommandContext(ctx, s.tesseractBinary, filePath, "stdout", "-l", "eng")
	// Set a reasonable timeout.
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	output, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "tesseract command failed")
	}
	return strings.TrimSpace(string(output)), nil
}

// runTesseractTSV executes tesseract to produce TSV output and parses it into WordConfidence structs.
func (s *Service) runTesseractTSV(ctx context.Context, filePath string) ([]WordConfidence, error) {
	// tesseract <input> stdout -l eng tsv
	cmd := exec.CommandContext(ctx, s.tesseractBinary, filePath, "stdout", "-l", "eng", "tsv")
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain stdout pipe")
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to start tesseract tsv command")
	}
	defer cmd.Wait()

	reader := csv.NewReader(bufio.NewReader(stdout))
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1 // allow variable columns

	// Skip header line.
	records, err := reader.ReadAll()
	if err != nil {
		return nil, errors.Wrap(err, "failed to read TSV output")
	}
	if len(records) < 2 {
		// No data rows.
		return []WordConfidence{}, nil
	}
	var results []WordConfidence
	for _, rec := range records[1:] {
		// Expected columns (as per tesseract TSV):
		// level	page_num	block_num	par_num	line_num	word_num	left	top	width	height	conf	text
		if len(rec) < 12 {
			continue // malformed line
		}
		confStr := rec[10]
		text := rec[11]

		// Skip empty text.
		if strings.TrimSpace(text) == "" {
			continue
		}
		conf, err := strconv.ParseFloat(confStr, 64)
		if err != nil {
			// Confidence may be "-1" for non‑word entries; ignore those.
			continue
		}
		results = append(results, WordConfidence{
			Text:       text,
			Confidence: conf,
		})
	}
	return results, nil
}

// calculateMeanConfidence computes the average confidence from a slice of WordConfidence.
// Returns 0 if the slice is empty.
func calculateMeanConfidence(words []WordConfidence) float64 {
	if len(words) == 0 {
		return 0
	}
	var sum float64
	for _, w := range words {
		sum += w.Confidence
	}
	return sum / float64(len(words))
}