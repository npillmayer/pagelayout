package template

import (
	"errors"
	"io/fs"
	"path/filepath"
)

// LoadFS opens and compiles one collection in an explicitly supplied filesystem.
// Name must be an io/fs path. It is also the diagnostic source name. LoadFS
// performs no repository search and closes the file, including on failure.
// Open, read, compilation and close failures return nil with a *LoadError;
// errors.Is/As preserve the underlying filesystem errors.
func LoadFS(fsys fs.FS, name string) (*Catalog, error) {
	file, err := openCollection(fsys, name, name)
	if err != nil {
		return nil, newLoadError([]diagnostic{*err})
	}
	return loadOpenedCollection(file, name)
}

// LoadFile locates and compiles a collection using host I/O. A nil hostIO uses
// the OS. Relative names are tried in UserConfigDir()/templates, then relative
// to the current directory. An unavailable configuration directory is skipped.
// Absolute paths are opened directly; relative paths escaping through ".." are
// resolved from the current directory only. No extension is added and no
// recursive directory scan or template-ID lookup is performed.
//
// Only a missing file permits fallback: an unreadable or invalid collection
// stops the search. Source references contain the selected candidate's path.
// Files are closed before return. Failures return nil with a *LoadError,
// preserving filesystem causes; if all candidates are missing, the diagnostics
// list the attempted paths in search order. No directories are created, and no
// configuration files or system-wide repository settings are read yet.
func LoadFile(name string, hostIO IO) (*Catalog, error) {
	if name == "" || filepath.Base(name) == "." || filepath.Base(name) == ".." ||
		(filepath.VolumeName(name) != "" && !filepath.IsAbs(name)) {
		return nil, newLoadError([]diagnostic{fileDiagnostic(codeInputPath, name, "invalid collection path",
			&fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid})})
	}
	if hostIO == nil {
		hostIO = systemIO{}
	}
	name = filepath.Clean(name)
	var missing []diagnostic
	if filepath.IsLocal(name) {
		if configDir, err := hostIO.UserConfigDir(); err == nil && configDir != "" {
			dir := filepath.Join(configDir, "templates")
			source := filepath.Join(dir, name)
			file, failure := openCollection(hostIO.DirFS(dir), filepath.ToSlash(name), source)
			if failure == nil {
				return loadOpenedCollection(file, source)
			}
			if !errors.Is(failure.cause, fs.ErrNotExist) {
				return nil, newLoadError([]diagnostic{*failure})
			}
			missing = append(missing, *failure)
		}
	}
	dir, base := filepath.Split(name)
	if dir == "" {
		dir = "."
	}
	file, failure := openCollection(hostIO.DirFS(filepath.Clean(dir)), base, name)
	if failure != nil {
		if !errors.Is(failure.cause, fs.ErrNotExist) {
			return nil, newLoadError([]diagnostic{*failure})
		}
		return nil, newLoadError(append(missing, *failure))
	}
	return loadOpenedCollection(file, name)
}

func openCollection(fsys fs.FS, name, source string) (fs.File, *diagnostic) {
	if fsys == nil || !fs.ValidPath(name) {
		d := fileDiagnostic(codeInputPath, source, "invalid collection filesystem or path",
			&fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid})
		return nil, &d
	}
	file, err := fsys.Open(name)
	if err != nil {
		d := fileDiagnostic(codeInputOpen, source, "cannot open collection", err)
		return nil, &d
	}
	return file, nil
}

func loadOpenedCollection(file fs.File, source string) (*Catalog, error) {
	catalog, err := Load(file, LoadOptions{SourceName: source})
	if closeErr := file.Close(); closeErr != nil {
		var ds []diagnostic
		if err != nil {
			// Load guarantees a *LoadError for every failed compilation.
			ds = err.(*LoadError).diagnostics()
		}
		ds = append(ds, fileDiagnostic(codeInputClose, source, "cannot close collection", closeErr))
		return nil, newLoadError(ds)
	}
	return catalog, err
}

func fileDiagnostic(code diagnosticCode, source, message string, cause error) diagnostic {
	return diagnostic{stage: stageIO, code: code, source: sourceRef{name: source}, message: message, cause: cause}
}
