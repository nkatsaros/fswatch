package fswatch

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type FileInfo struct {
	Path     string
	FileInfo os.FileInfo
}

type Watcher struct {
	Changed chan FileInfo
	stop    chan int
}

func WatchFile(filename string, frequency time.Duration) *Watcher {
	fw := &Watcher{make(chan FileInfo), make(chan int)}

	lastFi, _ := os.Stat(filename)

	go func() {
		ticker := time.NewTicker(frequency)
		changed := false

		// send a change signal initially
		if lastFi != nil {
			fw.Changed <- FileInfo{filename, lastFi}
		}

		for {
			select {
			case <-ticker.C:
				fi, err := os.Stat(filename)

				if err != nil {
					// If there's an error stating the file do nothing
					break
				}

				// check if the file did not originally exist
				if lastFi == nil {
					lastFi = fi
					changed = true
					break
				}

				wasChanged := fi.ModTime() != lastFi.ModTime() || fi.Size() != lastFi.Size()

				switch {
				// The file was modified
				case wasChanged:
					changed = true
				// The file was not modified since it was last changed
				case changed && !wasChanged && !fi.IsDir():
					// Send the newest stat to the channel
					fw.Changed <- FileInfo{filename, fi}
					// Reset the changed flag
					changed = false
				}

				lastFi = fi
			case _, ok := <-fw.stop:
				if !ok {
					ticker.Stop()
					return
				}
			}
		}
	}()

	return fw
}

func (fw *Watcher) Close() error {
	close(fw.stop)
	return nil
}

type dirFileInfo struct {
	fileInfo os.FileInfo
	changed  bool
	newFile  bool
}

func WatchDirectory(dirname string, frequency time.Duration) *Watcher {
	fw := &Watcher{make(chan FileInfo), make(chan int)}

	fileList := make(map[string]*dirFileInfo)

	go func() {
		walkAction := func(path string, fi os.FileInfo, err error) error {
			if dfi, ok := fileList[path]; ok && !fi.IsDir() && err == nil {
				lastFi := dfi.fileInfo
				wasChanged := fi.ModTime() != lastFi.ModTime() || fi.Size() != lastFi.Size()

				switch {
				case wasChanged:
					dfi.fileInfo = fi
					dfi.changed = true
				case !wasChanged && dfi.changed:
					dfi.changed = false
					fw.Changed <- FileInfo{path, fi}
				case !wasChanged && dfi.newFile:
					dfi.newFile = false
					fw.Changed <- FileInfo{path, fi}
				}

			} else {
				fileList[path] = &dirFileInfo{fi, false, true}
			}
			return nil
		}

		// do a first walk of the directory immediately to get initial results
		filepath.Walk(dirname, walkAction)

		ticker := time.NewTicker(frequency)

		for {
			select {
			case <-ticker.C:
				filepath.Walk(dirname, walkAction)
			case _, ok := <-fw.stop:
				if !ok {
					ticker.Stop()
					return
				}
			}
		}
	}()

	return fw
}
