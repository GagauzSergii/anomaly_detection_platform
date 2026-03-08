package detector

import (
	"math"
	"sort"
	"sync"
	"time"
)

type DetectionMode string

const (
	ModeUp   DetectionMode = "MODE_UP"
	ModeDown DetectionMode = "MODE_DOWN"
	ModeBoth               = "MODE_BOTH"
)

// Config defines MAD detector parameters.
type Config struct {
	WindowSize int           // Max number of recent values stored per series.
	Warmup     int           // Minimum number of samples required before detecting anomalies.
	ThresholdK float64       // Sensitivity multiplier: threshold = median + K * MAD.
	Mode       DetectionMode // Mode of detection to know spike direction up, down or both

	// MinMAD clamps MAD when series is flat (prevents hypersensitivity).
	// Units: same as the metric value.
	MinMad float64

	// MinDelta is a deadband around baselineMedian.
	// If |value-baseline| < MinDelta => not anomaly.
	// Units: same as the metric value.
	MinDelta float64
}

// MADDetector keeps in-memory sliding windows per metric series and detects anomalies.
// It is domain logic: no logging, no NATS, no time, no IO.
type MADDetector struct {
	config Config

	// FIXED: Concurrent access using RWMutex for map access
	mu              sync.RWMutex
	windowsBySeries map[SeriesKey]*slidingWindow
}

// slidingWindow stores the latest metric values for a series.
type slidingWindow struct {
	// FIXED: Fine-grained locking for window operations
	mu       sync.Mutex
	maxSize  int
	values   []float64
	lastSeen time.Time
}

// NewMADDetector creates a MAD detector with safe defaults.
func NewMADDetector(config Config) *MADDetector {
	if config.WindowSize <= 0 {
		config.WindowSize = 50
	}
	if config.Warmup <= 0 {
		config.Warmup = 20
	}
	if config.ThresholdK <= 0 {
		config.ThresholdK = 6
	}
	if config.Mode == "" {
		config.Mode = ModeUp
	}

	if config.MinMad <= 0 {
		config.MinMad = 1.0
	}

	if config.MinDelta < 0 {
		config.MinDelta = 0
	}

	if config.Warmup > config.WindowSize {
		config.Warmup = config.WindowSize
	}

	return &MADDetector{
		config:          config,
		windowsBySeries: make(map[SeriesKey]*slidingWindow),
	}
}

// Observe adds a value to the series window and returns statistics for anomaly decision.
// Returns isAnomaly=true only after Warmup samples have been collected.
func (detector *MADDetector) Observe(
	seriesKey SeriesKey,
	metricValue float64,
) (
	baselineMedian float64,
	deviationMAD float64,
	anomalyLowerThreshold float64,
	anomalyUpperThreshold float64,
	currentWindowSize int,
	isAnomaly bool) {

	// Concurrent access pattern using fine-grained locking
	seriesWindow := detector.getOrCreateWindow(seriesKey)

	// Lock only the specific window for processing
	seriesWindow.mu.Lock()
	defer seriesWindow.mu.Unlock()

	seriesWindow.append(metricValue)

	// Update last seen for cleanup
	seriesWindow.lastSeen = time.Now()

	currentWindowSize = len(seriesWindow.values)
	if currentWindowSize < detector.config.Warmup {
		// Not enough data to make a statistically meaningful decision.
		return 0, 0, 0, 0, currentWindowSize, false
	}

	// Baseline is robust median of the window.
	baselineMedian = median(seriesWindow.values)

	// MAD is robust spread estimator: median(|xi - baseline|).
	deviationMAD = medianAbsoluteDeviation(seriesWindow.values, baselineMedian)

	// If MAD is zero (flat series), threshold would equal baseline and
	// any tiny numeric jitter could become anomaly. We clamp with epsilon.
	//const epsilon = 1e-9
	//if deviationMAD < epsilon {
	//	deviationMAD = epsilon
	//}

	// Clamp MAD to MinMAD to avoid hypersensitivity on flatline.
	if deviationMAD < detector.config.MinMad {
		deviationMAD = detector.config.MinMad
	}

	// Convert MAD to sigma-like scale (normal distribution consistency)
	const madConsistencyConstant = 1.4826
	sigmaLike := madConsistencyConstant * deviationMAD

	delta := detector.config.ThresholdK * sigmaLike
	anomalyUpperThreshold = baselineMedian + delta
	anomalyLowerThreshold = baselineMedian - delta

	if detector.config.MinDelta > 0 {
		if math.Abs(metricValue-baselineMedian) < detector.config.MinDelta {
			return baselineMedian, deviationMAD, anomalyLowerThreshold, anomalyUpperThreshold,
				currentWindowSize, false
		}
	}

	switch detector.config.Mode {
	case ModeDown:
		isAnomaly = metricValue < anomalyLowerThreshold
	case ModeBoth:
		isAnomaly = metricValue > anomalyUpperThreshold || metricValue < anomalyLowerThreshold
	case ModeUp, "":
		isAnomaly = metricValue > anomalyUpperThreshold
	default:
		isAnomaly = metricValue > anomalyUpperThreshold
	}

	return baselineMedian, deviationMAD, anomalyLowerThreshold, anomalyUpperThreshold, currentWindowSize, isAnomaly
}

// getOrCreateWindow returns the series window, creating it if missing.
func (detector *MADDetector) getOrCreateWindow(seriesKey SeriesKey) *slidingWindow {
	// Optimistic read lock
	detector.mu.RLock()
	seriesWindow, exists := detector.windowsBySeries[seriesKey]
	detector.mu.RUnlock()

	if exists {
		return seriesWindow
	}

	// Write lock to create
	detector.mu.Lock()
	defer detector.mu.Unlock()

	// Double check
	seriesWindow, exists = detector.windowsBySeries[seriesKey]
	if exists {
		return seriesWindow
	}

	seriesWindow = &slidingWindow{
		maxSize:  detector.config.WindowSize,
		lastSeen: time.Now(),
	}
	detector.windowsBySeries[seriesKey] = seriesWindow

	return seriesWindow
}

// Cleanup removes windows that haven't been updated since maxAge.
func (detector *MADDetector) Cleanup(maxAge time.Duration) int {
	detector.mu.Lock()
	defer detector.mu.Unlock()

	now := time.Now()
	cleaned := 0

	for key, window := range detector.windowsBySeries {
		// Lock window to safely read lastSeen, preventing race with Observe
		window.mu.Lock()
		lastSeen := window.lastSeen
		window.mu.Unlock()

		if now.Sub(lastSeen) > maxAge {
			delete(detector.windowsBySeries, key)
			cleaned++
		}
	}
	return cleaned
}

// append pushes a new value and trims the oldest if the window exceeds max size.
func (window *slidingWindow) append(metricValue float64) {
	window.values = append(window.values, metricValue)

	if len(window.values) <= window.maxSize {
		return
	}

	// Drop the oldest value in-place to avoid reallocations.
	copy(window.values, window.values[1:])
	window.values = window.values[:window.maxSize]
}

// median returns the statistical median of the slice.
// Note: We copy + sort to keep the original window order unchanged.
func median(values []float64) float64 {
	sortedValues := make([]float64, len(values))
	copy(sortedValues, values)
	sort.Float64s(sortedValues)

	valuesCount := len(sortedValues)
	middleIndex := valuesCount / 2

	if valuesCount == 0 {
		return 0
	}
	if valuesCount%2 == 1 {
		return sortedValues[middleIndex]
	}
	return (sortedValues[middleIndex-1] + sortedValues[middleIndex]) / 2
}

// medianAbsoluteDeviation computes MAD: median(|xi - baseline|).
func medianAbsoluteDeviation(values []float64, baselineMedian float64) float64 {
	absoluteDeviations := make([]float64, len(values))
	for index, metricValue := range values {
		absoluteDeviations[index] = math.Abs(metricValue - baselineMedian)
	}
	return median(absoluteDeviations)
}
