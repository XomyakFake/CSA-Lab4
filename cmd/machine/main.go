package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/XomyakFake/CSA-Lab4/internal/isa"
	"github.com/XomyakFake/CSA-Lab4/internal/machine"
)

const maxTicks = 100_000_000 // чтобы зациклившаяся программа не работала вечно

const usage = "usage: machine <code.bin> <input.txt> [--no-cache] [--log-limit N]"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "machine:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	var files []string
	useCache := true
	logLimit := 0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--no-cache": // каждое обращение к памяти 10 тактов
			useCache = false
		case "--log-limit": // оставить начало и конец журнала, по N/2 строк
			i++
			if i == len(args) {
				return errors.New(usage)
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return fmt.Errorf("bad --log-limit value %q", args[i])
			}
			logLimit = n
		default:
			files = append(files, args[i])
		}
	}
	if len(files) != 2 {
		return errors.New(usage)
	}

	raw, err := os.ReadFile(files[0])
	if err != nil {
		return err
	}
	var prog isa.Program
	if err = prog.UnmarshalBinary(raw); err != nil {
		return fmt.Errorf("%s: %w", files[0], err)
	}
	input, err := os.ReadFile(files[1])
	if err != nil {
		return err
	}

	m := machine.New(prog, input, useCache)
	m.SetLog(os.Stderr, logLimit) // вывод программы в stdout, журнал в stderr
	m.Run(maxTicks)
	if _, err = os.Stdout.Write(m.IO.Output); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "stop: %s, instructions: %d, ticks: %d (waiting for memory: %d)\n",
		m.StopReason, m.Instructions, m.Ticks, m.WaitTicks)
	fmt.Fprintf(os.Stderr, "I-cache hits: %d, misses: %d, D-cache hits: %d, misses: %d\n",
		m.ICache.Hits, m.ICache.Misses, m.DCache.Hits, m.DCache.Misses)
	return nil
}
