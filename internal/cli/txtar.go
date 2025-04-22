package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/matthewmueller/dev/internal/matcher"
	"golang.org/x/sync/errgroup"
	"golang.org/x/tools/txtar"
)

type TxtarPack struct {
	Dir         string
	Includes    []string
	Excludes    []string
	NoGitIgnore bool
}

func (c *CLI) TxtarPack(ctx context.Context, in *TxtarPack) error {
	gitIgnore, err := c.gitIgnore(c.Dir, in.NoGitIgnore)
	if err != nil {
		return err
	}
	match, err := matcher.Compile(in.Includes, in.Excludes, gitIgnore)
	if err != nil {
		return err
	}
	absDir, err := c.resolve(in.Dir)
	if err != nil {
		return err
	}
	ar := &txtar.Archive{}
	if err := filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		} else if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".golden" || filepath.Ext(path) == ".txtar" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(absDir, path)
		if err != nil {
			return err
		}
		if !match(relPath) {
			return nil
		}
		ar.Files = append(ar.Files, txtar.File{
			Name: relPath,
			Data: append(bytes.TrimSpace(data), '\n', '\n'),
		})
		return nil
	}); err != nil {
		return err
	}
	// Write the archive to stdout
	fmt.Fprintln(c.Stdout, string(packArchive(ar)))
	return nil
}

type TxtarUnpack struct {
	Path        string
	Dir         string
	Force       bool
	Includes    []string
	Excludes    []string
	NoGitIgnore bool
}

func (c *CLI) TxtarUnpack(ctx context.Context, in *TxtarUnpack) error {
	gitIgnore, err := c.gitIgnore(c.Dir, in.NoGitIgnore)
	if err != nil {
		return err
	}
	match, err := matcher.Compile(in.Includes, in.Excludes, gitIgnore)
	if err != nil {
		return err
	}
	inputPath, err := c.resolve(in.Path)
	if err != nil {
		return err
	}
	absDir, err := c.resolve(in.Dir)
	if err != nil {
		return err
	}
	ar, err := unpackArchive(inputPath)
	if err != nil {
		return err
	}
	eg := &errgroup.Group{}
	for _, file := range ar.Files {
		file := file
		if !match(file.Name) {
			continue
		}
		eg.Go(func() error {
			if err := os.MkdirAll(filepath.Join(absDir, filepath.Dir(file.Name)), 0755); err != nil {
				return err
			}
			if err := os.WriteFile(filepath.Join(absDir, file.Name), file.Data, 0644); err != nil {
				return err
			}
			return nil
		})
	}
	return eg.Wait()
}

func isBinary(data []byte) bool {
	if len(data) > 0 {
		for _, b := range data {
			if b < 0x20 && b != '\n' && b != '\r' && b != '\t' {
				return true
			}
		}
	}
	return false
}

func toBase64(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var result string
	for i := 0; i < len(encoded); i += 80 {
		end := min(i+80, len(encoded))
		result += encoded[i:end] + "\n"
	}
	return result
}

func packArchive(ar *txtar.Archive) []byte {
	for i, file := range ar.Files {
		if isBinary(file.Data) {
			ar.Files[i].Data = []byte(toBase64(file.Data))
			ar.Files[i].Name = file.Name + " base64"
		}
	}
	return txtar.Format(ar)
}

func fromBase64(data string) ([]byte, error) {
	cleaned := ""
	for _, line := range strings.Split(data, "\n") {
		cleaned += line
	}
	return base64.StdEncoding.DecodeString(cleaned)
}

func unpackArchive(inputPath string) (*txtar.Archive, error) {
	ar, err := txtar.ParseFile(inputPath)
	if err != nil {
		return nil, err
	}
	for i := range ar.Files {
		if strings.HasSuffix(ar.Files[i].Name, " base64") {
			data, err := fromBase64(string(ar.Files[i].Data))
			if err != nil {
				return nil, err
			}
			ar.Files[i].Data = data
			ar.Files[i].Name = strings.TrimSuffix(ar.Files[i].Name, " base64")
		}
	}
	return ar, nil
}
