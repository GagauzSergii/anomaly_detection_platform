package detector

import (
	"sync"
	"testing"
	"time"
)

func TestNewMADDetector_Defaults(t *testing.T) {
	d := NewMADDetector(Config{})
	if d.config.WindowSize != 50 {
		t.Errorf("expected default WindowSize 50, got %d", d.config.WindowSize)
	}
	if d.config.Warmup != 20 {
		t.Errorf("expected default Warmup 20, got %d", d.config.Warmup)
	}
	if d.config.ThresholdK != 6 {
		t.Errorf("expected default ThresholdK 6, got %f", d.config.ThresholdK)
	}
	if d.config.Mode != ModeUp {
		t.Errorf("expected default Mode ModeUp, got %s", d.config.Mode)
	}
}

func TestObserve_Warmup(t *testing.T) {
	d := NewMADDetector(Config{
		WindowSize: 10,
		Warmup:     5,
	})

	key := SeriesKey{MetricName: "test_metric"}

	// First 4 samples -> no anomaly (warmup)
	for i := 0; i < 4; i++ {
		_, _, _, _, currentSize, isAnomaly := d.Observe(key, 10.0)
		if isAnomaly {
			t.Errorf("unexpected anomaly during warmup at index %d", i)
		}
		if currentSize != i+1 {
			t.Errorf("expected size %d, got %d", i+1, currentSize)
		}
	}
}

func TestObserve_Anomaly(t *testing.T) {
	d := NewMADDetector(Config{
		WindowSize: 10,
		Warmup:     5,
		ThresholdK: 2, // Low threshold for easy triggering
		Mode:       ModeUp,
		MinMad:     0.1,
	})

	key := SeriesKey{MetricName: "test_metric"}

	// Fill with stable values
	for i := 0; i < 9; i++ {
		d.Observe(key, 10.0)
	}

	// Next value: Spike
	_, _, _, upper, _, isAnomaly := d.Observe(key, 20.0)
	if !isAnomaly {
		t.Errorf("expected anomaly for value 20.0 with baselines around 10.0")
	}
	if upper >= 20.0 {
		t.Errorf("expected upper threshold < 20.0, got %f", upper)
	}
}

func TestCleanup(t *testing.T) {
	d := NewMADDetector(Config{WindowSize: 10})
	key1 := SeriesKey{MetricName: "old_metric"}
	key2 := SeriesKey{MetricName: "new_metric"}

	// Add old metric
	d.Observe(key1, 10.0)

	// Manually backdate the lastSeen time
	d.mu.Lock()
	if w, ok := d.windowsBySeries[key1]; ok {
		w.lastSeen = time.Now().Add(-2 * time.Hour)
	}
	d.mu.Unlock()

	// Add new metric
	d.Observe(key2, 10.0)

	// Cleanup older than 1 hour
	cleaned := d.Cleanup(1 * time.Hour)

	if cleaned != 1 {
		t.Errorf("expected 1 cleaned series, got %d", cleaned)
	}

	d.mu.RLock()
	defer d.mu.RUnlock()
	if _, ok := d.windowsBySeries[key1]; ok {
		t.Errorf("old_metric should have been removed")
	}
	if _, ok := d.windowsBySeries[key2]; !ok {
		t.Errorf("new_metric should still exist")
	}
}

func TestConcurrency(t *testing.T) {
	// This test is designed to run with -race
	d := NewMADDetector(Config{WindowSize: 50})
	var wg sync.WaitGroup

	// Many concurrent writers to different keys
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := SeriesKey{MetricName: "metric", Region: string(rune(id))}
			for j := 0; j < 100; j++ {
				d.Observe(key, float64(j))
			}
		}(i)
	}

	// Concurrent cleanup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			time.Sleep(10 * time.Millisecond)
			d.Cleanup(1 * time.Second)
		}
	}()

	wg.Wait()
}
