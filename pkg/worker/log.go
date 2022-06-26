package worker

import (
	"bufio"
	"io"
	"os"
	"strings"

	"github.com/fsnotify/fsnotify"
	"go.uber.org/zap"
)

type LogFile struct {
	file *os.File
}

func NewLogFile(filepath string) (*LogFile, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}

	return &LogFile{
		file: file,
	}, nil
}

func (l *LogFile) Follow(stopCh <-chan bool) (<-chan string, error) {
	logCh := make(chan string)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	err = watcher.Add(l.file.Name())
	if err != nil {
		return nil, err
	}

	go func() {
		defer func() {
			if closeErr := watcher.Close(); closeErr != nil {
				zap.L().Error("error closing watcher", zap.Error(closeErr))
			}
			close(logCh)
		}()

		// Read all logs on initial file creation
		if err = readAll(l.file, logCh); err != nil {
			zap.L().Error("error reading log file", zap.String("file", l.file.Name()), zap.Error(err))
		}
		// Watch for file write changes and read new lines
		if err = stream(l.file, watcher, logCh, stopCh); err != nil {
			zap.L().Error("error reading log file line", zap.String("file", l.file.Name()), zap.Error(err))
		}
	}()

	return logCh, nil
}

func (l *LogFile) Close() error {
	return l.file.Close()
}

func readLine(reader *bufio.Reader) (string, error) {
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}

	return strings.TrimRight(line, "\n"), nil
}

func readAll(file *os.File, lineCh chan<- string) error {
	reader := bufio.NewReader(file)
	for {
		line, err := readLine(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		lineCh <- line
	}
	return nil
}

func stream(file *os.File, watcher *fsnotify.Watcher, lineCh chan<- string, doneCh <-chan bool) error {
	reader := bufio.NewReader(file)
	for {
		select {
		case <-doneCh:
			return nil
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			if event.Op&fsnotify.Write == fsnotify.Write {
				line, err := readLine(reader)
				if err != nil && err != io.EOF {
					return err
				}
				lineCh <- line
			}
		case watchErr := <-watcher.Errors:
			return watchErr
		}
	}
}
