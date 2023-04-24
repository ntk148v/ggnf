<h1 align="center">GGNF - Go Get Nerd Fonts</h1>

<p align="center">
  <h4>A tool to manage and download <a href="https://github.com/ryanoasis/nerd-fonts">Nerd Fonts</a> quickly</h4>
</p>

<p align="center">
    <a href="https://github.com/ntk148v/ggnf/blob/master/LICENSE">
        <img alt="GitHub license" src="https://img.shields.io/github/license/ntk148v/ggnf?style=for-the-badge">
    </a>
    <a href="https://github.com/ntk148v/ggnf/stargazers"> <img alt="GitHub stars" src="https://img.shields.io/github/stars/ntk148v/ggnf?style=for-the-badge"> </a>
    <a href="https://github.com/ntk148v/ggnf/issues"><img src="https://img.shields.io/github/issues/ntk148v/ggnf?colorA=192330&colorB=dbc074&style=for-the-badge"></a>
    <a href="https://github.com/ntk148v/ggnf/contributors"><img src="https://img.shields.io/github/contributors/ntk148v/ggnf?colorA=192330&colorB=81b29a&style=for-the-badge"></a>
</p>

Table of content

- [1. Features](#1-features)
- [2. Installation](#2-installation)
- [3. Usage](#3-usage)
- [4. Contribution](#4-contribution)

## 1. Features

- Easily manage a list of Nerd Fonts with corresponding version.
- Able to download multiple fonts at once.
- A progress bar!

## 2. Installation

- Download the latest binary from the [Release page]. It's the easiest way to get started with `ggnf`.
- Give it execute permission.

```shell
$ chmod a+x ggnf
```

- Make sure to add the location of the binary to your `$PATH`. Or simply, put it to the right place:

```shell
$ sudo mv ggnf /usr/local/bin/ggnf
```

## 3. Usage

```shell
$ ggnf
ggnf is Nerd Font downloader written in Golang.
<https://github.com/ntk148v/ggnf>

Usage:
  ggnf list                           - List all fonts
  ggnf download <font1> <font2> ...   - Download the given fonts
  ggnf remove <font1> <font2> ...     - Remove the given fonts

```

- GGNF will check for the Nerd Fonts latest release. It may take a few seconds (depend on your network).

```shell
$ ggnf list
# Output
...
        },
        "UbuntuMono": {
                "name": "UbuntuMono",
                "download_url": "https://github.com/ryanoasis/nerd-fonts/releases/download/v2.3.3/UbuntuMono.zip",
                "installed": "",
                "latest": "v2.3.3"
        },
        "VictorMono": {
                "name": "VictorMono",
                "download_url": "https://github.com/ryanoasis/nerd-fonts/releases/download/v2.3.3/VictorMono.zip",
                "installed": "v2.3.3",
                "latest": "v2.3.3"
        },
        "iA-Writer": {
                "name": "iA-Writer",
                "download_url": "https://github.com/ryanoasis/nerd-fonts/releases/download/v2.3.3/iA-Writer.zip",
                "installed": "v2.3.3",
                "latest": "v2.3.3"
        }
}

# If you feel this list is too long, use your Linux skill
$ ggnf list | less
```

- By default, `ggnf` will download and save font to $FONT directory:

  - User: `$HOME/.local/share/fonts`
  - Root (run with root or `sudo`): `/usr/local/share/fonts`

- Download new fonts:

```shell
$ ggnf download UbuntuMono Iosevka
Found new release: v2.3.3
Initializing GGNF for the first time ...
Downloading Iosevka                2% |██                                                                                                      | (17/653 MB, 640 kB/s) [36s:16m57s]
Downloading UbuntuMono           (17 MB, 29 MB/s) [0s]                                                                                       | (15/653 MB, 751 kB/s) [32s:14m30s]
```

## 4. Contribution

Feel free to file an issue or open a pull request. You're welcome!
