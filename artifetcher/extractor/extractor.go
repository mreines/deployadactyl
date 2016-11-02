// Package extractor unzips artifacts.
package extractor

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"

	"github.com/op/go-logging"
	"github.com/spf13/afero"
)

const niceFixYourZipMessage = `Please double check your zip compression method and that the correct files are zipped.
You can try confirming that it's valid on your computer by opening or performing some other action on it.
Once you've confirmed that it's valid, please try again.`

// Extractor has a file system from which files are extracted from.
type Extractor struct {
	Log        *logging.Logger
	FileSystem *afero.Afero
}

// Unzip unzips from source into destination.
// If there is no manifest provided to this function, it will attempt to read a manifest file within the zip file.
func (e *Extractor) Unzip(source, destination, manifest string) error {
	e.Log.Info("extracting application")
	e.Log.Debug(`parameters for extractor:
	source: %+v
	destination: %+v`, source, destination)

	err := e.FileSystem.MkdirAll(destination, 0755)
	if err != nil {
		return fmt.Errorf("cannot create directory: %s", err)
	}

	file, err := e.FileSystem.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()

	fileStat, err := file.Stat()
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(file, fileStat.Size())
	if err != nil {
		return fmt.Errorf("cannot open zip file: %s: %s\n%s", source, err, niceFixYourZipMessage)
	}

	for _, file := range reader.File {
		err := e.unzipFile(destination, file)
		if err != nil {
			return fmt.Errorf("cannot extract file from archive: %s: %s", file.Name, err)
		}
	}

	if manifest != "" {
		manifestFile, err := e.FileSystem.OpenFile(path.Join(destination, "manifest.yml"), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return fmt.Errorf("cannot open manifest file: %s", err)
		}
		defer manifestFile.Close()

		_, err = fmt.Fprint(manifestFile, manifest)
		if err != nil {
			return fmt.Errorf("cannot print to open manifest file: %s", err)
		}
	}

	e.Log.Info("extract was successful")
	return nil
}

func (e *Extractor) unzipFile(destination string, file *zip.File) error {
	contents, err := file.Open()
	if err != nil {
		return fmt.Errorf("cannot extract file from archive: %s", err)
	}
	defer contents.Close()

	if file.FileInfo().IsDir() {
		return nil
	}

	savedLocation := path.Join(destination, file.Name)
	directory := path.Dir(savedLocation)
	err = e.FileSystem.MkdirAll(directory, 0755)
	if err != nil {
		return fmt.Errorf("cannot make directory: %s: %s", directory, err)
	}

	mode := file.Mode()
	newFile, err := e.FileSystem.OpenFile(savedLocation, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("cannot open file for writing: %s: %s", savedLocation, err)
	}
	defer newFile.Close()

	_, err = io.Copy(newFile, contents)
	if err != nil {
		return fmt.Errorf("cannot write to file: %s: %s", savedLocation, err)
	}

	return nil
}
