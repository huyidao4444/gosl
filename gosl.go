package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/daviddengcn/go-villa"
)

const (
	STAGE_READY = iota
	STAGE_IMPORT
	STAGE_MAIN
)

func genFilename(suffix villa.Path) villa.Path {
	if !strings.HasSuffix(suffix.S(), ".go") {
		suffix += ".go"
	}
	dir := villa.Path(os.TempDir())
	for {
		base := villa.Path(fmt.Sprintf("gosl-%08x-%s", rand.Int63n(math.MaxInt64), suffix))
		fn := dir.Join(base)
		if !fn.Exists() {
			return fn
		}
	}
}

func execCode(err error) int {
	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return 0
}

var (
	DEFAULT_IMPORT = []string {
		"fmt", "Printf",
		"os", "Exit",
		"strings", "Contains",
		"github.com/daviddengcn/gosl/builtin", "Exec",
	}
)

func process() error {
	fn := villa.Path(os.Args[1])
	buffer, err := ioutil.ReadFile(fn.S())
	if err != nil {
		return err
	}

	var code bytes.Buffer
	code.WriteString(`package main;`)
	for i := 0; i < len(DEFAULT_IMPORT); i += 2 {
		code.WriteString(` import . "`)
		code.WriteString(DEFAULT_IMPORT[i])
		code.WriteString(`";`)
	}
	code.WriteString(`func init() {`)
	for i := 1; i < len(DEFAULT_IMPORT); i += 2 {
		code.WriteString(` _ = `)
		code.WriteString(DEFAULT_IMPORT[i])
		code.WriteString(`;`)
	}
	code.WriteString(` } `)

	stage := STAGE_READY

	buf := buffer
	for len(buf) > 0 {
		p := bytes.IndexByte(buf, byte('\n'))
		var line []byte
		if p < 0 {
			line = buf
			buf = nil
		} else {
			line = buf[:p]
			buf = buf[p+1:]
		}

		if len(line) == 0 {
			if _, err := code.WriteRune('\n'); err != nil {
				return err
			}
			continue
		}

		for {
			switch stage {
			case STAGE_READY:
				if line[0] != '#' {
					stage = STAGE_IMPORT
					continue
				}
				line = nil
			case STAGE_IMPORT:
				if !bytes.HasPrefix(line, []byte("import ")) {
					stage = STAGE_MAIN
				}
			}
			break
		}

		if stage == STAGE_MAIN {
			if _, err := code.WriteString("func main() { "); err != nil {
				return err
			}
		}
		if _, err := code.Write(line); err != nil {
			return err
		}
		if _, err := code.WriteRune('\n'); err != nil {
			return err
		}
		if stage == STAGE_MAIN {
			break
		}
	}
	if stage == STAGE_MAIN {
		if _, err := code.Write(buf); err != nil {
			return err
		}
	} else {
		if _, err := code.WriteString("\nfunc main() { "); err != nil {
			return err
		}
	}

	if _, err := code.WriteString("\n}\n"); err != nil {
		return err
	}

	codeFn := genFilename(fn.Base())
	if err := codeFn.WriteFile(code.Bytes(), 0644); err != nil {
		return err
	}
	defer codeFn.Remove()

	exeFn := codeFn + ".exe"

	cmd := villa.Path("go").Command("build", "-o", exeFn.S(), codeFn.S())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return err
	}
	defer exeFn.Remove()

	cmd = exeFn.Command(os.Args[2:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	err = cmd.Run()
	ec := execCode(err)
	if ec != 0 {
		os.Exit(ec)
	}
	return err
}

func main() {
	if len(os.Args) < 2 {
		return
	}

	if err := process(); err != nil {
		fmt.Printf("Failed: %v\n", err)
	}
}