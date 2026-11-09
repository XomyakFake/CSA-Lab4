package golden

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/XomyakFake/CSA-Lab4/internal/isa"
	"github.com/XomyakFake/CSA-Lab4/internal/machine"
	"github.com/XomyakFake/CSA-Lab4/internal/translator"
)

var update = flag.Bool("update", false, "rewrite golden files with the actual results") // go test ./golden -update

const maxTicks = 5_000_000

type goldenCase struct {
	Source       string `yaml:"source"` // должен совпадать с examples/<имя>.alg
	Input        string `yaml:"input"`
	LogLimit     int    `yaml:"log_limit"` // 0 значит весь журнал
	AST          string `yaml:"ast"`
	Code         string `yaml:"code"`
	Data         string `yaml:"data"`
	Output       string `yaml:"output"`
	Stop         string `yaml:"stop"`
	Instructions int    `yaml:"instructions"`
	Ticks        int    `yaml:"ticks"`
	TicksNoCache int    `yaml:"ticks_no_cache"`
	Log          string `yaml:"log"`
}

func TestGolden(t *testing.T) {
	files, err := filepath.Glob("*.yaml")
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden files found: %v", err)
	}
	for _, file := range files {
		name := strings.TrimSuffix(file, ".yaml")
		t.Run(name, func(t *testing.T) {
			raw, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var want goldenCase
			if err = yaml.Unmarshal(raw, &want); err != nil {
				t.Fatalf("%s: %v", file, err)
			}
			source, err := os.ReadFile(filepath.Join("..", "examples", name+".alg"))
			if err != nil {
				t.Fatal(err)
			}
			got := runCase(t, string(source), want.Input, want.LogLimit)

			if *update {
				if err = os.WriteFile(file, marshal(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			compare(t, "source", got.Source, want.Source)
			compare(t, "ast", got.AST, want.AST)
			compare(t, "code", got.Code, want.Code)
			compare(t, "data", got.Data, want.Data)
			compare(t, "output", got.Output, want.Output)
			compare(t, "stop", got.Stop, want.Stop)
			compare(t, "instructions", strconv.Itoa(got.Instructions), strconv.Itoa(want.Instructions))
			compare(t, "ticks", strconv.Itoa(got.Ticks), strconv.Itoa(want.Ticks))
			compare(t, "ticks_no_cache", strconv.Itoa(got.TicksNoCache), strconv.Itoa(want.TicksNoCache))
			compare(t, "log", got.Log, want.Log)
		})
	}
}

func runCase(t *testing.T, source, input string, logLimit int) goldenCase {
	t.Helper()
	res, err := translator.Translate(source)
	if err != nil {
		t.Fatalf("translate: %v", err)
	}
	// гоняем программу через бинарный формат, как будто она пришла из файла
	bin, err := res.Program.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	var prog isa.Program
	if err = prog.UnmarshalBinary(bin); err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer
	cached := machine.New(prog, []byte(input), true)
	cached.SetLog(&log, logLimit)
	cached.Run(maxTicks)

	direct := machine.New(prog, []byte(input), false) // без кеша, меняться должно только число тактов
	direct.Run(maxTicks)
	if !bytes.Equal(cached.IO.Output, direct.IO.Output) || cached.Instructions != direct.Instructions {
		t.Errorf("results with and without cache differ: %q (%d instructions) and %q (%d instructions)",
			cached.IO.Output, cached.Instructions, direct.IO.Output, direct.Instructions)
	}

	return goldenCase{
		Source:       source,
		Input:        input,
		LogLimit:     logLimit,
		AST:          res.AST,
		Code:         res.CodeListing,
		Data:         res.DataListing,
		Output:       string(cached.IO.Output),
		Stop:         cached.StopReason,
		Instructions: cached.Instructions,
		Ticks:        cached.Ticks,
		TicksNoCache: direct.Ticks,
		Log:          log.String(),
	}
}

func compare(t *testing.T, field, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		g, w := "<no line>", "<no line>"
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			t.Errorf("%s differs at line %d:\n  got:  %s\n  want: %s\n(run go test ./golden -update to accept)", field, i+1, g, w)
			return
		}
	}
}

func marshal(c goldenCase) []byte {
	doc := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key, tag, value string) {
		node := &yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value}
		if strings.Contains(value, "\n") {
			node.Style = yaml.LiteralStyle // многострочное блоком через |, так файл читается глазами
		}
		doc.Content = append(doc.Content, &yaml.Node{Kind: yaml.ScalarNode, Value: key}, node)
	}
	add("source", "!!str", c.Source)
	add("input", "!!str", c.Input)
	add("log_limit", "!!int", strconv.Itoa(c.LogLimit))
	add("ast", "!!str", c.AST)
	add("code", "!!str", c.Code)
	add("data", "!!str", c.Data)
	add("output", "!!str", c.Output)
	add("stop", "!!str", c.Stop)
	add("instructions", "!!int", strconv.Itoa(c.Instructions))
	add("ticks", "!!int", strconv.Itoa(c.Ticks))
	add("ticks_no_cache", "!!int", strconv.Itoa(c.TicksNoCache))
	add("log", "!!str", c.Log)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		panic(fmt.Sprintf("encode golden file: %v", err))
	}
	return buf.Bytes()
}
