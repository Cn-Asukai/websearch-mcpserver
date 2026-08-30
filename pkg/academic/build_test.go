package academic

import (
	"slices"
	"testing"

	"websearch/pkg/antirobot"
	"websearch/pkg/proxy"
)

func buildOpts(europePMC, dblp, doaj bool) struct {
	Network         antirobot.NetworkRegion
	Arxiv           antirobot.ArxivOpts
	Crossref        antirobot.CrossrefOpts
	OpenAlex        antirobot.OpenAlexOpts
	SemanticScholar antirobot.SemanticScholarOpts
	PubMed          antirobot.PubMedOpts
	GoogleScholar   antirobot.GoogleScholarOpts
	EuropePMC       antirobot.EuropePMCOpts
	DBLP            antirobot.DBLPOpts
	DOAJ            antirobot.DOAJOpts
	ProxyResolve    proxy.ProxyResolver
} {
	return struct {
		Network         antirobot.NetworkRegion
		Arxiv           antirobot.ArxivOpts
		Crossref        antirobot.CrossrefOpts
		OpenAlex        antirobot.OpenAlexOpts
		SemanticScholar antirobot.SemanticScholarOpts
		PubMed          antirobot.PubMedOpts
		GoogleScholar   antirobot.GoogleScholarOpts
		EuropePMC       antirobot.EuropePMCOpts
		DBLP            antirobot.DBLPOpts
		DOAJ            antirobot.DOAJOpts
		ProxyResolve    proxy.ProxyResolver
	}{
		Network:   antirobot.RegionChina,
		EuropePMC: antirobot.EuropePMCOpts{Enabled: europePMC},
		DBLP:      antirobot.DBLPOpts{Enabled: dblp},
		DOAJ:      antirobot.DOAJOpts{Enabled: doaj},
	}
}

// TestBuildAcademic_NewEngines 新增的 Europe PMC / DBLP / DOAJ 应注册为国内引擎，
// 对应 Enabled 置 false 时不注册。
func TestBuildAcademic_NewEngines(t *testing.T) {
	all := BuildAcademic(buildOpts(true, true, true))
	names := make([]string, 0, len(all))
	for _, e := range all {
		names = append(names, e.Name())
	}
	for _, want := range []string{"europepmc", "dblp", "doaj"} {
		if !slices.Contains(names, want) {
			t.Errorf("引擎 %s 应被注册, got %v", want, names)
		}
	}

	none := BuildAcademic(buildOpts(false, false, false))
	names2 := make([]string, 0, len(none))
	for _, e := range none {
		names2 = append(names2, e.Name())
	}
	for _, banned := range []string{"europepmc", "dblp", "doaj"} {
		if slices.Contains(names2, banned) {
			t.Errorf("禁用后不应注册 %s, got %v", banned, names2)
		}
	}
}
