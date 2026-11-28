package credentialstore

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// ReadPrivateFile rejects symlinks, nonregular files, permissive modes, and
// oversized records. It is also used for nonsecret provider identity metadata.
func ReadPrivateFile(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 || info.Size() > maxFileBytes {
		return nil, errors.New("credential files must be regular, private, and bounded")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxFileBytes+1))
	if err != nil {
		return nil, errors.New("cannot read credential file")
	}

	if len(data) > maxFileBytes {
		return nil, errors.New("credential file is too large")
	}

	return data, nil
}

// WritePrivateFile atomically replaces a file using a synced, private temporary
// file and syncs its directory. Callers must hold the directory's credential lock.
func WritePrivateFile(path string, data []byte) error {
	if len(data) > maxFileBytes {
		return errors.New("credential file is too large")
	}

	f, err := os.CreateTemp(filepath.Dir(path), ".credentials-*")
	if err != nil {
		return errors.New("cannot create credential file")
	}
	defer os.Remove(f.Name())
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return errors.New("cannot write credential file")
	}

	if err := f.Sync(); err != nil {
		return errors.New("cannot sync credential file")
	}

	if err := f.Close(); err != nil {
		return errors.New("cannot close credential file")
	}

	if err := os.Rename(f.Name(), path); err != nil {
		return errors.New("cannot replace credential file")
	}

	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return errors.New("cannot open credential directory for sync")
	}
	defer dir.Close()

	return dir.Sync()
}
