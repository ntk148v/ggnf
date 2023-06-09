package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/google/go-github/github"
	"github.com/schollz/progressbar/v3"
)

var (
	helpText = `
ggnf is Nerd Font downloader written in Golang.
<https://github.com/ntk148v/ggnf>

Usage:
  ggnf list                           - List all fonts
  ggnf download <font1> <font2> ...   - Download the given fonts
  ggnf remove <font1> <font2> ...     - Remove the given fonts

`
	// colorize output
	infoPrint  = color.New(color.FgGreen).PrintfFunc()
	warnPrint  = color.New(color.FgYellow).PrintfFunc()
	errorPrint = color.New(color.FgRed).PrintfFunc()
)

func main() {
	configDir, _ := os.UserConfigDir()
	dataFile := filepath.Join(configDir, "ggnf.json")

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get font dir
	fontDir := getFontDir()

	// Load data
	fonts, err := loadData(dataFile)
	if err != nil {
		errorPrint("Unable to load data from file due to: %s\n", err)
		os.Exit(1)
	}

	defer saveData(dataFile, fonts)

	// Get Nerd Fonts latest release from Github
	if err := getLatestRelease(ctx, fonts); err != nil {
		errorPrint("Unable to get latest Nerd Fonts release due to: %s\n", err)
		os.Exit(1)
	}

	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "list":
			infoPrint("List all Nerd Fonts with version\n")
			if err := printJSON(fonts); err != nil {
				errorPrint("Unable to list fonts due to: %s\n", err)
				os.Exit(1)
			}
		case "download":
			// Create MultiProgressBar
			mpb := NewMultiProgressBar(make([]*progressbar.ProgressBar, 0), os.Stderr)

			var wg sync.WaitGroup

			for _, a := range args[1:] {
				wg.Add(1)
				go func(font string) {
					defer wg.Done()
					f, ok := fonts[font]
					if !ok {
						warnPrint("Unable to find font %s, make sure you enter the correct font\n", font)
						return
					}
					if f.InstalledVersion == f.LatestVersion {
						infoPrint("Font %s already installed, skip ...\n", font)
						return
					}

					if err := downloadFont(mpb, fonts[font], fontDir); err != nil {
						errorPrint("Unable to download font %s due to: %s\n", font, err)
						return
					}

					// Update installed version
					f.InstalledVersion = f.LatestVersion
					fonts[font] = f
				}(a)
			}
			wg.Wait()

			if err := scanFontDir(fontDir); err != nil {
				errorPrint("Error when scanning the font directory %s and building font information cache files: %s\n", fontDir, err)
				return
			}

			infoPrint("All done!")
		case "remove":
			var wg sync.WaitGroup
			for _, a := range args[1:] {
				wg.Add(1)
				go func(font string) {
					defer wg.Done()
					f, ok := fonts[font]
					if !ok {
						warnPrint("Unable to find font %s, make sure you enter the correct font\n", font)
						return
					}

					// Remove fonts
					infoPrint("Removing font %s ...\n", font)
					if err := removeFont(f, fontDir); err != nil {
						errorPrint("Error when removing font %s: %s \n", font, err)
						return
					}
					// Update installed version
					f.InstalledVersion = ""
					fonts[font] = f
				}(a)
			}
			wg.Wait()

			if err := scanFontDir(fontDir); err != nil {
				errorPrint("Error when scanning the font directory %s and building font information cache files: %s\n", fontDir, err)
			}
		case "-h", "--help", "help":
			infoPrint(helpText)
		}
	} else {
		infoPrint(helpText)
	}
}

type Font struct {
	Name             string `json:"name"`
	DownloadURL      string `json:"download_url"`
	InstalledVersion string `json:"installed"`
	LatestVersion    string `json:"latest"`
}

type LineWriter struct {
	*MultiProgressBar
	id int
}

func (lw *LineWriter) Write(p []byte) (n int, err error) {
	lw.guard.Lock()
	defer lw.guard.Unlock()
	lw.Move(lw.id, lw.output)
	return lw.output.Write(p)
}

type MultiProgressBar struct {
	output  io.Writer
	curLine int
	Bars    []*progressbar.ProgressBar
	guard   sync.Mutex
}

func NewMultiProgressBar(pBars []*progressbar.ProgressBar, output io.Writer) *MultiProgressBar {
	mpb := &MultiProgressBar{
		curLine: 0,
		Bars:    pBars,
		guard:   sync.Mutex{},
		output:  output,
	}
	for id, pb := range mpb.Bars {
		progressbar.OptionSetWriter(&LineWriter{
			MultiProgressBar: mpb,
			id:               id,
		})(pb)
	}

	return mpb
}

func (mpb *MultiProgressBar) Add(b *progressbar.ProgressBar) {
	mpb.Bars = append(mpb.Bars, b)
	id := len(mpb.Bars) - 1
	progressbar.OptionSetWriter(&LineWriter{
		MultiProgressBar: mpb,
		id:               id,
	})(b)
}

// Move cursor to the beginning of the current progressbar.
func (mpb *MultiProgressBar) Move(id int, writer io.Writer) (int, error) {
	bias := mpb.curLine - id
	mpb.curLine = id
	if bias > 0 {
		// move up
		return fmt.Fprintf(writer, "\r\033[%dA", bias)
	} else if bias < 0 {
		// move down
		return fmt.Fprintf(writer, "\r\033[%dB", -bias)
	}
	return 0, nil
}

// End Move cursor to the end of the Progressbars.
func (mpb *MultiProgressBar) End() {
	mpb.Move(len(mpb.Bars), mpb.output)
}

// printJSON prints v as JSON encoded with indent to stdout. It panics on any error.
func printJSON(v interface{}) error {
	w := json.NewEncoder(os.Stdout)
	w.SetIndent("", "\t")
	return w.Encode(v)
}

// getLatestRelease fetches Github for the latest Nerd Fonts release
func getLatestRelease(ctx context.Context, fonts map[string]Font) error {
	// Github client
	client := github.NewClient(nil)
	latestRelease, _, err := client.Repositories.GetLatestRelease(ctx, "ryanoasis", "nerd-fonts")
	if err != nil {
		return err
	}

	// Randomly get the first font to check version
	if latestRelease.GetName() == fonts["3270"].LatestVersion {
		// Skip cause this is the already latest release
		return nil
	}

	infoPrint("Found new release: %s\n", latestRelease.GetName())
	infoPrint("Initializing GGNF for the first time ...")
	for _, a := range latestRelease.Assets {
		f := Font{
			Name:          strings.TrimSuffix(a.GetName(), ".zip"),
			LatestVersion: latestRelease.GetName(),
			DownloadURL:   a.GetBrowserDownloadURL(),
		}
		if tmp, ok := fonts[f.Name]; ok {
			f.InstalledVersion = tmp.InstalledVersion
		}

		fonts[f.Name] = f
	}

	return nil
}

// isRoot checks whether the current user is root or not
func isRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		errorPrint("Unable to get current user: %s\n", err)
		os.Exit(1)
	}
	return currentUser.Username == "root"
}

// getFontDir gets font directory
func getFontDir() string {
	var dir string

	switch runtime.GOOS {
	case "windows":

	case "darwin", "ios":

	case "plan9":

	default: // Unix
		if isRoot() {
			dir = "/usr/local/share/fonts"
		} else {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, ".local/share/fonts")
		}
	}

	// Create font dir
	dir = filepath.Join(dir, "NerdFonts")
	_ = os.MkdirAll(dir, os.ModePerm)
	return dir
}

// removeFont deletes the font directory
func removeFont(font Font, fontDir string) error {
	return os.RemoveAll(filepath.Join(fontDir, font.Name))
}

// downloadFont gets the font from Github release and extract
// to the right place
func downloadFont(mpb *MultiProgressBar, font Font, fontDir string) error {
	archivePath := filepath.Join(os.TempDir(), font.Name+".zip")
	// Create the file
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}

	resp, err := http.Get(font.DownloadURL)
	if err != nil {
		return err
	}

	defer func() {
		resp.Body.Close()
		out.Close()
		os.Remove(archivePath)
	}()

	bar := progressbar.NewOptions64(
		resp.ContentLength,
		progressbar.OptionSetDescription(fmt.Sprintf("Downloading %-20s", font.Name)),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(10),
		progressbar.OptionThrottle(50*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprintf(os.Stderr, "\nDownloaded %-20s\n", font.Name)
		}),
		progressbar.OptionFullWidth(),
		progressbar.OptionSetRenderBlankState(false),
	)

	// Add to MultiProgressBar
	mpb.Add(bar)

	// Writer the body to file
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		return err
	}

	// Unzip
	return unzip(archivePath, filepath.Join(fontDir, font.Name))
}

// scanFontDir
func scanFontDir(fontDir string) error {
	infoPrint("Scan font dir to build cache")
	switch runtime.GOOS {
	case "windows":

	case "darwin", "ios":

	case "plan9":

	default: // Unix
		// Run fc-cache to update font list (Linux). Don't know how it works in Darwin, Windows
		cmd := exec.Command("fc-cache", "-f", fontDir)
		return cmd.Run()
	}
	return nil
}

// loadData gets the list of fonts
func loadData(dataFile string) (map[string]Font, error) {
	fonts := make(map[string]Font, 0)
	if _, err := os.Stat(dataFile); err != nil {
		if os.IsNotExist(err) {
			_, _ = os.Create(dataFile)
			return fonts, nil
		}
		return fonts, err
	}

	raw, err := os.ReadFile(dataFile)
	if err != nil {
		return fonts, err
	}

	_ = json.Unmarshal(raw, &fonts)
	return fonts, nil
}

// saveData dumps the list of fonts to disk
func saveData(dataFile string, fonts map[string]Font) error {
	raw, err := json.MarshalIndent(fonts, "", "	")
	if err != nil {
		return err
	}

	return os.WriteFile(dataFile, raw, 0644)
}

// unzip - get from https://stackoverflow.com/questions/20357223/easy-way-to-unzip-file-with-golang
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	_ = os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(path, f.Mode())
		} else {
			_ = os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}
