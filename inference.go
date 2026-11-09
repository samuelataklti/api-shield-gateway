package main

import (
	"fmt"
	"github.com/dmitryikh/leaves"
	"log"
	"math"
	"os"
	"sync"
	"time"
)

var (
	lgbmModel      *leaves.Ensemble
	predictionPool sync.Pool
)

type Prediction struct {
	Label     int     `json:"label"`
	Score     float64 `json:"score"`
	ClassName string  `json:"class_name"`
	Blocked   bool    `json:"blocked"`
}

type PredictionTrace struct {
	Prediction Prediction
	Inference  time.Duration
	TotalModel time.Duration
}

func setupLightGBMLeaves() {
	modelPath := getenv("LGBM_MODEL_PATH", "api_threat_model_lgbm_antioverfit_strict_12_v3.txt")

	if _, err := os.Stat(modelPath); err != nil {
		log.Fatalf("LightGBM model file not found: %s", modelPath)
	}

	useTransformation := getenv("LEAVES_TRANSFORM", "1") == "1"

	model, err := leaves.LGEnsembleFromFile(modelPath, useTransformation)
	if err != nil {
		log.Fatalf("Failed to load LightGBM model: %v", err)
	}

	if model.NFeatures() != featureCount {
		log.Fatalf("Model expects %d features, but gateway uses %d", model.NFeatures(), featureCount)
	}

	if model.NOutputGroups() < 4 {
		log.Fatalf("Model output groups must be >= 4, got %d", model.NOutputGroups())
	}

	lgbmModel = model

	predictionPool = sync.Pool{
		New: func() any {
			return make([]float64, model.NOutputGroups())
		},
	}

	fmt.Printf("[+] Loaded LightGBM model: %s\n", modelPath)
	fmt.Printf("[+] Model name=%s features=%d outputs=%d estimators=%d transform=%v\n",
		model.Name(), model.NFeatures(), model.NOutputGroups(), model.NEstimators(), useTransformation)
}
func predictWithLeavesTimed(f Features) PredictionTrace {
	totalStart := time.Now()

	values := featuresToSlice(f)

	predAny := predictionPool.Get()
	preds := predAny.([]float64)
	for i := range preds {
		preds[i] = 0
	}
	defer predictionPool.Put(preds)

	inferStart := time.Now()
	err := lgbmModel.Predict(values, 0, preds)
	inference := time.Since(inferStart)

	if err != nil {
		if debugLogs {
			fmt.Printf("[MODEL ERROR] %v\n", err)
		}
		return PredictionTrace{
			Prediction: Prediction{
				Label:     labelBenign,
				Score:     1,
				ClassName: "BENIGN",
				Blocked:   false,
			},
			Inference:  inference,
			TotalModel: time.Since(totalStart),
		}
	}

	bestLabel := 0
	bestScore := math.Inf(-1)

	for i := 0; i < len(preds) && i < 4; i++ {
		if preds[i] > bestScore {
			bestScore = preds[i]
			bestLabel = i
		}
	}

	p := Prediction{
		Label:     bestLabel,
		Score:     bestScore,
		ClassName: labelName(bestLabel),
		Blocked:   shouldBlock(bestLabel, bestScore),
	}

	return PredictionTrace{
		Prediction: p,
		Inference:  inference,
		TotalModel: time.Since(totalStart),
	}
}
func featuresToSlice(f Features) []float64 {
	return []float64{
		f.LogReqSize,
		f.IsReadMethod,
		f.IsWriteMethod,
		f.EndpointDepth,
		f.EndpointHasIDPlaceholder,
		f.PayloadPresent,
		f.LogPayloadLength,
		f.PayloadShapeScore,
		f.PayloadNonAlnumRatio,
		f.RepeatedEndpointAccessCountSoFar,
		f.ResourceProbeRateSoFar,
		f.TargetChangeRateSoFar,
	}
}
func shouldBlock(label int, score float64) bool {
	if label == labelBenign {
		return false
	}
	return score >= thresholdForLabel(label)
}
func thresholdForLabel(label int) float64 {
	switch label {
	case labelBOLA:
		return getenvFloat("THRESHOLD_BOLA", 0.50)
	case labelBFLA:
		return getenvFloat("THRESHOLD_BFLA", 0.50)
	case labelSQLi:
		return getenvFloat("THRESHOLD_SQLI", 0.50)
	default:
		return 0.50
	}
}
func labelName(label int) string {
	switch label {
	case labelBenign:
		return "BENIGN"
	case labelBOLA:
		return "BOLA"
	case labelBFLA:
		return "BFLA"
	case labelSQLi:
		return "SQLi"
	default:
		return "UNKNOWN"
	}
}
