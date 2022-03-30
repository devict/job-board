package lorem

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

func init() {
	w := map[string]bool{}
	for i := range words {
		if _, ok := w[words[i]]; ok {
			panic(fmt.Sprintf("duplicate word: %s", words[i]))
		}
		w[words[i]] = true
	}
}

var words = [...]string{"atmospheric", "then", "nonfungible", "actionable", "wholesome", "visibility", "insights", "disrupt", "database", "dramatic", "must", "market", "elevate", "days", "weeks", "week", "collaborate", "meta", "tesla", "bottom-line", "isomer", "people", "parks", "semantic", "ergo", "because", "which", "therefor", "remote", "unique", "web 3.0", "synergy", "clicks-and-mortar", "golang", "360-degree", "target", "uniquely", "project", "proactive", "agile", "domination", "key", "quickly", "continually", "wireless", "rapaciously", "thinking", "expertise", "synergistically", "tactical", "line", "compelling", "implement", "cross-unit", "core", "expedite", "distinctive", "leverage", "organically", "workflows", "installed", "base", "compliant", "reintermediate", "robust", "end", "directed", "functional", "others", "offshoring", "long-term", "close", "podcasting", "networks", "resource-leveling", "an", "outsourcing", "capitalize", "state", "granular", "premium", "items", "and", "procrastinate", "empower", "assertively", "expanded", "generate", "ballpark", "eye", "sound", "bricks-and-clicks", "effective", "view", "workplace", "your", "roi", "other", "by", "frameworks", "inside", "high-impact", "pontificate", "procedures", "fully", "theme", "communicate", "leadership", "evolved", "testing", "low-risk", "learn", "scale", "array", "digital", "cross-platform", "offline", "on", "override", "clickthroughs", "completely", "multimedia", "bleeding-edge", "maximizing", "functionalized", "for", "proposition", "underwhelm", "monotonically", "service", "web-readiness", "opportunities", "inexpensive", "build", "corporate", "manufactured", "via", "content", "e-tailers", "cost", "strategies", "nanotechnology", "resource", "whereas", "alignments", "unleash", "cross-media", "art", "envisioned", "fabricate", "framework", "superior", "idea-sharing", "maximise", "coordinate", "disruptive", "immersion", "information", "high-yield", "high", "normal", "in", "with", "holisticly", "process-centric", "competently", "integration", "ideas", "competencies", "provide", "collaborative", "timely", "quality", "embrace", "frictionless", "level", "relationships", "high-payoff", "deploy", "prospective", "reinvent", "multiple", "based", "covalent", "standardized", "has", "benefits", "engage", "scalable", "vectors", "whiteboard", "heading", "experiences", "applications", "deliver", "enable", "fruit", "researched", "plagiarize", "compellingly", "principle-centered", "focused", "next-generation", "is", "products", "enabled", "identify", "incubate", "e-business", "interfaces", "e-commerce", "technically", "foster", "at", "interactively", "adaptive", "distinctively", "progressively", "appropriately", "mentality", "actualize", "methods", "matrix", "going", "added", "pursue", "top-line", "virtual", "initiatives", "additional", "focusing", "markets", "proactively", "results", "taking", "market-driven", "seamless", "after", "visionary", "further", "integrated", "diverse", "long", "evolve", "customer", "credibly", "best", "bottom", "premier", "web", "infrastructures", "enterprise", "mailchimp", "loop", "runway", "performing", "transparent", "that", "backend", "customized", "or", "re-engineer", "process", "metrics", "data", "accurate", "intrinsically", "productize", "cooperative", "generation", "highway", "time", "client-centric", "myocardinate", "cultivate", "solutions", "iterate", "infomediaries", "client-focused", "existing", "towards", "before", "without", "phosfluorescently", "turnkey", "vertical", "integrate", "strategy", "of", "web-enabled", "approaches", "empowered", "synthesize", "skills", "business", "ubiquitous", "plug-and-play", "efficiently", "indicators", "users", "dynamically", "dramatically", "maximize", "one-to-one", "objectively", "task", "paradigms", "table", "streamlined", "energistically", "iterative", "niche", "growth", "intellectual", "imperatives", "incentivize", "network", "overviews", "value", "holistic", "cloud", "user", "schemas", "innovate", "initiate", "about", "practices", "processes", "exploit", "areas", "from", "synergize", "outside", "models", "just", "materials", "world-class", "world", "establish", "through", "sustainable", "optimal", "technologies", "distributed", "devops", "holistically", "user-centric", "diversity", "solution", "potentialities", "divide", "leading-edge", "management", "aggregate", "touchpoints", "backward-compatible", "technology", "to", "deliverables", "economically", "empowerment", "interoperable", "ethical", "tested", "the", "change", "chains", "vis-a-vis", "scenarios", "win-win", "generated", "have", "drive", "rather", "cutting-edge", "box", "interdependent", "improvements", "ball", "standards", "innovation", "day", "future-proof", "strategic", "friendly", "start-up", "worldwide", "sources", "disseminate", "meta-services", "action", "taxing", "extensive", "dynamic", "more", "derive", "seize", "emerging", "overall", "forward", "hanging", "dive", "platforms", "utilize", "reconceptualize", "enthusiastically", "operational", "convergence", "visualize", "capital", "evisculate", "harness", "maintain", "ensure", "along", "extensible", "seamlessly", "niches", "organic", "survival", "performance", "keeping", "collaboration", "than", "front-end", "inter-mandated", "beta", "tail", "while", "deep", "intuitive", "communities", "fashion", "professionally", "predominate", "bring", "will", "collaboratively", "supply", "portals", "mesh", "this", "enterprise-wide", "synopsis", "bandwidth", "highly", "solely", "administrate", "revolutionary", "parallel", "reliable", "architectures", "revolutionize", "grow", "internal", "impactful", "globally", "interactive", "sponsored", "new", "real-time", "test", "methodologies", "channels", "e-markets", "global", "open-source", "low", "activity", "services", "maintainable", "e-services", "catalysts", "efficient"}
var punctuation = [...]string{".", "!", "?"}

func Word() string {
	i, err := rand.Int(rand.Reader, big.NewInt(int64(len(words))))
	if err != nil {
		panic(err)
	}
	return words[i.Int64()]
}

func WordsN(n int) string {
	w := make([]string, n)
	for i := 0; i < n; i++ {
		w[i] = Word()
	}

	return strings.Join(w, " ")
}

func WordsRange(min, max int) string {
	numWords, err := rand.Int(rand.Reader, big.NewInt(int64(max-1)))
	if err != nil {
		panic(err)
	}

	n := numWords.Int64() + int64(min)

	w := make([]string, n)
	for i := int64(0); i < n; i++ {
		w[i] = Word()
	}

	return strings.Join(w, " ")
}

func Sentence() string {
	l, err := rand.Int(rand.Reader, big.NewInt(int64(20)))
	if err != nil {
		panic(err)
	}

	length := l.Int64() + 1

	w := strings.Split(WordsN(int(length)), " ")
	w[0] = strings.Title(w[0])

	i, err := rand.Int(rand.Reader, big.NewInt(int64(len(punctuation))))
	if err != nil {
		panic(err)
	}

	return strings.Join(w, " ") + punctuation[i.Int64()]
}

func Paragraph() string {
	l, err := rand.Int(rand.Reader, big.NewInt(int64(6)))
	if err != nil {
		panic(err)
	}

	length := l.Int64() + 1

	s := make([]string, length)
	for i := 0; i < int(length); i++ {
		s[i] = Sentence()
	}

	return strings.Join(s, " ")
}

func ParagraphsN(n int) string {
	p := make([]string, n)
	for i := 0; i < n; i++ {
		p[i] = Paragraph()
	}

	return strings.Join(p, "\n\n")
}

func URL() string {
	return fmt.Sprintf("https://%s.com/%s", Word(), Word())
}

func Email() string {
	return fmt.Sprintf("%s.%s@%s.com", Word(), Word(), Word())
}
