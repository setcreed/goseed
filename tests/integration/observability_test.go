package integration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/setcreed/goseed/internal/templates"
)

func TestMetricsProjectsCompileReadonly(t *testing.T) {
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		t.Run(transport, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: "none", Transport: transport, Metrics: true})
			manifest := map[string][]byte{}
			for _, name := range []string{"go.mod", "go.sum"} {
				data, err := os.ReadFile(filepath.Join(directory, name))
				if err != nil {
					t.Fatal(err)
				}
				manifest[name] = data
			}
			run(t, directory, "go", "test", "-mod=readonly", "-run", "^$", "./...")
			for name, original := range manifest {
				current, err := os.ReadFile(filepath.Join(directory, name))
				if err != nil || string(current) != string(original) {
					t.Errorf("generated %s changed during readonly verification", name)
				}
			}
			run(t, directory, "go", "test", "-mod=readonly", "./internal/pkg/observability")
		})
	}
}

func TestTracingProjectsCompileReadonly(t *testing.T) {
	for _, transport := range []string{"http", "grpc", "dual", "gateway"} {
		t.Run(transport, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: "none", Transport: transport, Tracing: true})
			verifyObservabilityReadonly(t, directory)
			run(t, directory, "go", "test", "-mod=readonly", "./internal/pkg/observability")
		})
	}
}

func TestCombinedObservabilityProjectsCompileReadonly(t *testing.T) {
	for _, transport := range []string{"http", "gateway"} {
		t.Run(transport, func(t *testing.T) {
			directory := renderProjectWith(t, templates.Options{Database: "none", Transport: transport, Metrics: true, Tracing: true})
			verifyObservabilityReadonly(t, directory)
			run(t, directory, "go", "test", "-mod=readonly", "./internal/pkg/observability")
		})
	}
}

func TestObservabilityRaceAndOverheadBaseline(t *testing.T) {
	directory := renderProjectWith(t, templates.Options{Database: "none", Transport: "http", Metrics: true, Tracing: true})
	raceOutput := run(t, directory, "go", "test", "-race", "-mod=readonly", "./internal/pkg/observability")
	t.Logf("directed observability race: %s", raceOutput)
	benchmarkOutput := run(t, directory, "go", "test", "-mod=readonly", "-run", "^$", "-bench", "Benchmark(MetricsRecord|TracingSpan)$", "-benchmem", "-benchtime=1s", "./internal/pkg/observability")
	t.Logf("observability overhead baseline: %s", benchmarkOutput)
}

func verifyObservabilityReadonly(t *testing.T, directory string) {
	t.Helper()
	manifest := map[string][]byte{}
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		manifest[name] = data
	}
	run(t, directory, "go", "test", "-mod=readonly", "-run", "^$", "./...")
	for name, original := range manifest {
		current, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || string(current) != string(original) {
			t.Errorf("generated %s changed during readonly verification", name)
		}
	}
}
