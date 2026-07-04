# AudioBook Tools

A command-line utility designed to download audiobooks from supported platforms and seamlessly assemble them into single, optimized `.m4b` files with embedded metadata, cover art, and chapter markers.

## Supported Platforms

- [Книга в Ухе (knigavuhe.org)](https://knigavuhe.org)
- [Дети Онлайн (deti-online.com)](https://deti-online.com)
- [Яндекс Книги (books.yandex.ru)](https://books.yandex.ru)

## Why this project?

Downloading audiobooks manually and organizing dozens of individual MP3 files is tedious and messy. This tool automates the entire pipeline, giving you full control depending on your needs:

- **Download only:** Fetch all MP3s, the book cover, and generate proper OPF metadata in one fast, concurrent command.
- **Build only:** If you already have a folder with MP3 files, the tool can compile them into a perfectly tagged `.m4b` file ready for modern audiobook players (Apple Books, Prologue, Audiobookshelf). During this process, MP3 files are automatically transcoded to M4A (AAC) at 64 kbps. This preserves pristine audiobook voice quality while significantly reducing the final file size compared to higher-bitrate MP3 originals.
- **All-in-one:** Download the book and automatically assemble it into a space-saving `.m4b` file, cleaning up intermediate MP3s to free up disk space.

## Requirements

- **Downloading:** No external dependencies are required.
- **Building:** To use the `build` feature (or the `-m4b` flag during download), **FFmpeg** and **FFprobe** must be installed and available in your system's `PATH`. Without them, audio conversion and `.m4b` assembly will not work. You can download them from the [official FFmpeg website](https://ffmpeg.org/download.html).

## Installation

Installation is simple: download the archive for your platform, unpack it into the directory where you want to run the tool, and start the binary.

Official releases are available on the [GitHub Releases page](https://github.com/underhax/audiobook-tools/releases).

1. Download the archive (`.tar.gz` for macOS/Linux, `.zip` for Windows) for your system.
2. Unpack the downloaded archive into the directory where you want to keep the utility.
3. Open a terminal (or command prompt) and run the utility from that folder.

## Usage

*The examples below assume you have downloaded the correct executable for your platform (e.g., `audiobook-tools` for macOS/Linux or `audiobook-tools.exe` for Windows).*

### 1. Download an Audiobook

Downloads the audiobook files to your local drive.

```bash
/path/to/audiobook-tools download -url "<AUDIOBOOK_URL>" [OPTIONS]
```

**Options:**
- `-url <string>`: **(Required)** URL of the audiobook to download.
- `-out <string>`: Output directory for the downloaded files (default ".").
- `-workers <int>`: Number of concurrent download workers (default 5).
- `-m4b`: Build M4B file after downloading.
- `-clean`: Clean up downloaded MP3 files after building M4B (only if `-m4b` is set).
- `-cover`: Download cover image (default true).
- `-metadata`: Create OPF metadata file (default true).
- `-deti-online-voice-version <int>`: Voice version to download (deti-online.com only) (default 1).
- `-debug`: Show ffmpeg output and warnings.

**Example (Download Only):**
```bash
/path/to/audiobook-tools download -url "<AUDIOBOOK_URL>" -out "~/Downloads"
```

**Example (Download, Build M4B, and Clean up MP3s):**
```bash
/path/to/audiobook-tools download -url "<AUDIOBOOK_URL>" -out "~/Downloads" -m4b -clean
```

### 2. Build an M4B File

If you already have a directory containing audiobook `.mp3` files, you can assemble them into an `.m4b` container. The tool will use the `metadata.opf` and `cover.jpg` inside the directory if they are present.

```bash
/path/to/audiobook-tools build -dir "/path/to/audiobook/directory" [OPTIONS]
```

**Options:**
- `-dir <string>`: **(Required)** Path to the directory containing the audiobook files.
- `-clean`: Clean up downloaded MP3 files after building M4B.
- `-debug`: Show ffmpeg output and warnings.

**Example:**
```bash
/path/to/audiobook-tools build -dir "~/Downloads/Author Name/Book Title" -clean
```

### 3. Authentication (for some platforms)

Some platforms, such as Яндекс Книги, require authentication to access audiobook data. You can save your authentication token using the `auth` command. The token will be securely saved in your system's default configuration directory and automatically used during downloads.

*For Яндекс Книги, you can find instructions on how to get an OAuth token in the [Yandex ID Documentation](https://yandex.ru/dev/id/doc/ru/tokens/debug-token).*

```bash
/path/to/audiobook-tools auth <provider> <token>
```

**Example:**
```bash
/path/to/audiobook-tools auth books_yandex my_secret_token_here
```

Alternatively, you can provide the token via an environment variable directly when downloading:
```bash
BOOKS_YANDEX_TOKEN=my_secret_token_here /path/to/audiobook-tools download -url "<AUDIOBOOK_URL>"
```
