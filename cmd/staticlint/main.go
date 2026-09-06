package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"go-musthave-metrics/cmd/staticlint/osexit"

	"github.com/kisielk/errcheck/errcheck"
	"github.com/timakin/bodyclose/passes/bodyclose"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/multichecker"
	"golang.org/x/tools/go/analysis/passes/appends"
	"golang.org/x/tools/go/analysis/passes/asmdecl"
	"golang.org/x/tools/go/analysis/passes/assign"
	"golang.org/x/tools/go/analysis/passes/atomic"
	"golang.org/x/tools/go/analysis/passes/atomicalign"
	"golang.org/x/tools/go/analysis/passes/bools"
	"golang.org/x/tools/go/analysis/passes/buildtag"
	"golang.org/x/tools/go/analysis/passes/cgocall"
	"golang.org/x/tools/go/analysis/passes/composite"
	"golang.org/x/tools/go/analysis/passes/copylock"
	"golang.org/x/tools/go/analysis/passes/deepequalerrors"
	"golang.org/x/tools/go/analysis/passes/defers"
	"golang.org/x/tools/go/analysis/passes/directive"
	"golang.org/x/tools/go/analysis/passes/errorsas"
	"golang.org/x/tools/go/analysis/passes/fieldalignment"
	"golang.org/x/tools/go/analysis/passes/framepointer"
	"golang.org/x/tools/go/analysis/passes/httpmux"
	"golang.org/x/tools/go/analysis/passes/httpresponse"
	"golang.org/x/tools/go/analysis/passes/ifaceassert"
	"golang.org/x/tools/go/analysis/passes/loopclosure"
	"golang.org/x/tools/go/analysis/passes/lostcancel"
	"golang.org/x/tools/go/analysis/passes/nilfunc"
	"golang.org/x/tools/go/analysis/passes/nilness"
	"golang.org/x/tools/go/analysis/passes/printf"
	"golang.org/x/tools/go/analysis/passes/reflectvaluecompare"
	"golang.org/x/tools/go/analysis/passes/shadow"
	"golang.org/x/tools/go/analysis/passes/shift"
	"golang.org/x/tools/go/analysis/passes/sigchanyzer"
	"golang.org/x/tools/go/analysis/passes/slog"
	"golang.org/x/tools/go/analysis/passes/sortslice"
	"golang.org/x/tools/go/analysis/passes/stdmethods"
	"golang.org/x/tools/go/analysis/passes/stdversion"
	"golang.org/x/tools/go/analysis/passes/stringintconv"
	"golang.org/x/tools/go/analysis/passes/structtag"
	"golang.org/x/tools/go/analysis/passes/testinggoroutine"
	"golang.org/x/tools/go/analysis/passes/tests"
	"golang.org/x/tools/go/analysis/passes/timeformat"
	"golang.org/x/tools/go/analysis/passes/unmarshal"
	"golang.org/x/tools/go/analysis/passes/unreachable"
	"golang.org/x/tools/go/analysis/passes/unsafeptr"
	"golang.org/x/tools/go/analysis/passes/unusedresult"
	"golang.org/x/tools/go/analysis/passes/unusedwrite"
	"golang.org/x/tools/go/analysis/passes/waitgroup"

	"honnef.co/go/tools/analysis/lint"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"
)

//go:embed errcheck_excludes.txt
var errcheckExcludes []byte

func getStandardAnalyzers() []*analysis.Analyzer {
	return []*analysis.Analyzer{
		appends.Analyzer,
		asmdecl.Analyzer,
		assign.Analyzer,
		atomic.Analyzer,
		atomicalign.Analyzer,
		bools.Analyzer,
		buildtag.Analyzer,
		cgocall.Analyzer,
		composite.Analyzer,
		copylock.Analyzer,
		deepequalerrors.Analyzer,
		defers.Analyzer,
		directive.Analyzer,
		errorsas.Analyzer,
		fieldalignment.Analyzer,
		framepointer.Analyzer,
		httpmux.Analyzer,
		httpresponse.Analyzer,
		ifaceassert.Analyzer,
		loopclosure.Analyzer,
		lostcancel.Analyzer,
		nilfunc.Analyzer,
		nilness.Analyzer,
		printf.Analyzer,
		reflectvaluecompare.Analyzer,
		shadow.Analyzer,
		shift.Analyzer,
		sigchanyzer.Analyzer,
		slog.Analyzer,
		sortslice.Analyzer,
		stdmethods.Analyzer,
		stdversion.Analyzer,
		stringintconv.Analyzer,
		structtag.Analyzer,
		testinggoroutine.Analyzer,
		tests.Analyzer,
		timeformat.Analyzer,
		unmarshal.Analyzer,
		unreachable.Analyzer,
		unsafeptr.Analyzer,
		unusedresult.Analyzer,
		unusedwrite.Analyzer,
		waitgroup.Analyzer,
	}
}

func getStaticcheckAnalyzers() []*analysis.Analyzer {
	result := unwrapLintAnalyzers(staticcheck.Analyzers)

	stylecheckNames := map[string]bool{
		"ST1000": true,
		"ST1005": true,
		"ST1020": true,
	}

	for _, a := range stylecheck.Analyzers {
		if stylecheckNames[a.Analyzer.Name] {
			result = append(result, a.Analyzer)
		}
	}

	return result
}

func unwrapLintAnalyzers(list []*lint.Analyzer) []*analysis.Analyzer {
	result := make([]*analysis.Analyzer, 0, len(list))
	for _, a := range list {
		if a != nil && a.Analyzer != nil {
			result = append(result, a.Analyzer)
		}
	}
	return result
}

func configureErrcheck() error {
	excludePath := filepath.Join(
		os.TempDir(),
		"go-musthave-metrics-errcheck-excludes.txt",
	)

	if err := os.WriteFile(excludePath, errcheckExcludes, 0o600); err != nil {
		return fmt.Errorf("write errcheck excludes: %w", err)
	}

	if err := errcheck.Analyzer.Flags.Set("exclude", excludePath); err != nil {
		return fmt.Errorf("set errcheck exclude file: %w", err)
	}

	return nil
}

func main() {
	analyzers := getStandardAnalyzers()
	analyzers = append(analyzers, getStaticcheckAnalyzers()...)

	if err := configureErrcheck(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to configure errcheck: %v\n", err)
		return
	}

	analyzers = append(analyzers, errcheck.Analyzer)
	analyzers = append(analyzers, bodyclose.Analyzer)
	analyzers = append(analyzers, osexit.Analyzer)

	multichecker.Main(analyzers...)
}
