package audit

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/hypnotox/agentic-workflows/internal/testsupport/gitfixture"
)

const benchmarkAuditLock = `{"awfVersion":"v0.18.0","schemaVersion":31,"files":{}}`

type auditHistoryBenchmarkFixture struct {
	root string
	base string
	head string
}

func BenchmarkAuditHistoryCodeOnly50(b *testing.B) {
	benchmarkAuditHistory(b, buildCodeOnlyAuditHistoryFixture)
}

func BenchmarkAuditHistoryAuthorityHeavy50(b *testing.B) {
	benchmarkAuditHistory(b, buildAuthorityHeavyAuditHistoryFixture)
}

func BenchmarkAuditHistoryMergeHeavy50(b *testing.B) {
	benchmarkAuditHistory(b, buildMergeHeavyAuditHistoryFixture)
}

func benchmarkAuditHistory(b *testing.B, build func(*testing.B) auditHistoryBenchmarkFixture) {
	fixture := build(b)
	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	benchmarkAuditHistorySetup(b, ctx, fixture)

	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		findings, _, err := Run(ctx, fixture.root, fixture.base, fixture.head, Inputs{Settings: Settings{UncommittedChanges: true}})
		if err != nil {
			b.Fatal(err)
		}
		if len(findings) != 0 {
			b.Fatalf("audit findings = %#v", findings)
		}
	}
}

func benchmarkAuditHistorySetup(b *testing.B, ctx context.Context, fixture auditHistoryBenchmarkFixture) {
	findings, count, err := Run(ctx, fixture.root, fixture.base, fixture.head, Inputs{Settings: Settings{UncommittedChanges: true}})
	if err != nil {
		b.Fatal(err)
	}
	if count < 50 {
		b.Fatalf("commit count = %d, want at least 50", count)
	}
	if len(findings) != 0 {
		b.Fatalf("setup audit findings = %#v", findings)
	}
}

func buildCodeOnlyAuditHistoryFixture(b *testing.B) auditHistoryBenchmarkFixture {
	repo, fixture := newAuditHistoryBenchmarkFixture(b)
	for i := range 50 {
		fixture.head = gitfixture.Commit(b, repo, fmt.Sprintf("feat(awf): code change %02d", i), map[string]string{
			fmt.Sprintf("internal/unrelated_%02d.go", i): "package unrelated\n\nconst Value = " + strconv.Itoa(i) + "\n",
		})
	}
	return fixture
}

func buildAuthorityHeavyAuditHistoryFixture(b *testing.B) auditHistoryBenchmarkFixture {
	repo, fixture := newAuditHistoryBenchmarkFixture(b)
	for i := range 50 {
		fixture.head = gitfixture.Commit(b, repo, fmt.Sprintf("docs(adr): add authority %02d", i), map[string]string{
			fmt.Sprintf("docs/decisions/%04d-authority.md", i+1): benchmarkAuditLegacyADR(i + 1),
		})
	}
	return fixture
}

func buildMergeHeavyAuditHistoryFixture(b *testing.B) auditHistoryBenchmarkFixture {
	repo, fixture := newAuditHistoryBenchmarkFixture(b)
	main := fixture.base
	for i := range 25 {
		main = gitfixture.Commit(b, repo, fmt.Sprintf("feat(awf): main change %02d", i), map[string]string{
			fmt.Sprintf("internal/main_%02d.go", i): "package mainhistory\n",
		})
	}

	gitfixture.CheckoutNewBranch(b, repo, "feature", fixture.base)
	feature := fixture.base
	for i := range 25 {
		write := map[string]string{fmt.Sprintf("internal/feature_%02d.go", i): "package featurehistory\n"}
		if i == 0 {
			write["docs/decisions/0001-feature.md"] = benchmarkAuditLegacyADR(1)
		}
		feature = gitfixture.Commit(b, repo, fmt.Sprintf("feat(awf): feature change %02d", i), write)
	}
	fixture.head = gitfixture.Merge(b, repo, "Merge feature\n\nAWF-Allow-Version: legacy\nAWF-Allow-Reason: integrated legacy authority", main, feature)
	return fixture
}

func newAuditHistoryBenchmarkFixture(b *testing.B) (gitfixture.Fixture, auditHistoryBenchmarkFixture) {
	repo := gitfixture.InitRepo(b)
	base := gitfixture.Commit(b, repo, "feat(awf): base", map[string]string{
		".awf/awf.lock":    benchmarkAuditLock,
		".awf/config.yaml": "prefix: benchmark\nintegrationBranch: master\ntargets: [claude]\n",
	})
	return repo, auditHistoryBenchmarkFixture{root: repo.Root(), base: base, head: base}
}

func benchmarkAuditLegacyADR(number int) string {
	return fmt.Sprintf("---\nstatus: Implemented\ndate: 2026-01-01\n---\n# ADR-%04d: Benchmark authority\n", number)
}
