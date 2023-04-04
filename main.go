package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/github"
)

var (
	cacheFile string
	cache     map[string]string
	items     []list.Item
	docStyle  = lipgloss.NewStyle().Margin(1, 2)
)

type item struct {
	name      string
	url       string
	latest    string
	installed string
}

func (i item) Title() string { return i.name }
func (i item) Description() string {
	return fmt.Sprintf("Installed version: %s - Latest version: %s", i.installed, i.latest)
}
func (i item) LatestVersion() string  { return i.latest }
func (i item) CurrentVersion() string { return i.installed }
func (i item) DownloadURL() string    { return i.url }
func (i item) FilterValue() string    { return i.name }

type listKeyMap struct {
	installFont   key.Binding
	uninstallFont key.Binding
}

func newListKeyMap() *listKeyMap {
	return &listKeyMap{
		installFont: key.NewBinding(
			key.WithKeys("a"),
			key.WithHelp("a", "install the newest version"),
		),
		uninstallFont: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "uninstall the chosen font"),
		),
	}
}

type model struct {
	list list.Model
	keys *listKeyMap
}

func newModel() model {
	listKeys := newListKeyMap()
	fonts := list.New(items, list.NewDefaultDelegate(), 0, 0)
	fonts.Title = "GGNF"
	fonts.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			listKeys.installFont,
			listKeys.uninstallFont,
		}
	}
	return model{
		list: fonts,
		keys: listKeys,
	}
}

func (m model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}

		// Don't match any of the keys below if we're actively filtering.
		if m.list.FilterState() == list.Filtering {
			break
		}

		switch {
		case key.Matches(msg, m.keys.installFont):
			// Download font
			font, _ := m.list.SelectedItem().(item)
			statusMsg := "Download " + font.name
			if err := downloadFont(font); err != nil {
				statusMsg += (" failed due to: " + err.Error())
			}

			return m, tea.Batch(m.list.NewStatusMessage(statusMsg))
		case key.Matches(msg, m.keys.uninstallFont):
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return docStyle.Render(m.list.View())
}

func main() {
	err := createCacheFile()
	if err != nil {
		panic(err)
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the latest release Nerd Fonts
	if err := getLatestRelease(ctx); err != nil {
		panic(err)
	}

	if _, err := tea.NewProgram(newModel()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

// createCacheFile
func createCacheFile() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}

	cacheFile = filepath.Join(cacheDir, "ggnf.json")
	if _, err := os.Stat(cacheFile); err != nil {
		if os.IsNotExist(err) {
			os.Create(cacheFile)
			return setCache()
		}
		return err
	}

	return nil
}

// getCache loads cache data from cache file
func getCache() error {
	raw, err := ioutil.ReadFile(cacheFile)
	if err != nil {
		return nil
	}

	return json.Unmarshal(raw, &cache)
}

// setCache saves the input to cache file
func setCache() error {
	raw, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(cacheFile, raw, 0644)
}

// getLatestRelease fetches Github for the latest Nerd Fonts release
func getLatestRelease(ctx context.Context) error {
	// Load installed fonts
	if err := getCache(); err != nil {
		return err
	}

	// Github client
	client := github.NewClient(nil)
	latestRelease, _, err := client.Repositories.GetLatestRelease(ctx, "ryanoasis", "nerd-fonts")
	if err != nil {
		return err
	}

	for _, a := range latestRelease.Assets {
		i := item{
			name:      strings.Trim(*a.Name, ".zip"),
			url:       *a.BrowserDownloadURL,
			latest:    *latestRelease.Name,
			installed: "None",
		}

		if v, ok := cache[i.name]; ok {
			i.installed = v
		}

		items = append(items, i)
	}
	return nil
}

func downloadFont(font item) error {
	tmpPath := filepath.Join(os.TempDir(), font.name+".zip")
	tmp, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	defer tmp.Close()
	// Download to tmp file
	resp, err := http.Get(font.url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Copy to tmp file
	_, err = io.Copy(tmp, resp.Body)
	if err != nil {
		return err
	}

	var fontDir string
	if os.Getuid() == 0 {
		fontDir = systemFontDir()
	} else {
		fontDir = userFontDir()
	}

	return unzip(tmpPath, filepath.Join(fontDir, font.name))
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, f.Mode())
		} else {
			var fdir string
			if lastIndex := strings.LastIndex(fpath, string(os.PathSeparator)); lastIndex > -1 {
				fdir = fpath[:lastIndex]
			}

			err = os.MkdirAll(fdir, f.Mode())
			if err != nil {
				return err
			}
			f, err := os.OpenFile(
				fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
