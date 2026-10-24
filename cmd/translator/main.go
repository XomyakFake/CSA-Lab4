package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/XomyakFake/CSA-Lab4/internal/isa"
	"github.com/XomyakFake/CSA-Lab4/internal/translator"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "translator:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) < 2 || len(args) > 3 {
		return errors.New("usage: translator <source> <out.bin> [<out.lst>]")
	}
	src, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	res, err := translator.Translate(string(src))
	if err != nil {
		return fmt.Errorf("%s: %w", args[0], err)
	}
	bin, err := res.Program.MarshalBinary()
	if err != nil {
		return err
	}
	if err := os.WriteFile(args[1], bin, 0o644); err != nil {
		return err
	}
	if len(args) == 3 {
		if err := os.WriteFile(args[2], []byte(res.Listing()), 0o644); err != nil {
			return err
		}
	}
	fmt.Printf("code: %d words, data: %d words\n", len(res.Program.Code), len(res.Program.Data)-isa.StaticBase/4)
	return nil
}
