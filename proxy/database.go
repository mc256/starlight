/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/mc256/starlight/util"
	"path"
	"time"
)

type Database struct {
	db *sql.DB
}

func (d *Database) Close() {
	_ = d.db.Close()
}

func NewDatabase(conStr string) (*Database, error) {
	d, err := sql.Open("postgres", conStr)
	if err != nil {
		return nil, err
	}
	return &Database{db: d}, nil
}

func (d *Database) InitDatabase() {
	// TODO: initialize database
}

func (d *Database) SetImageReady(ready bool, serial int) error {
	var id int64
	if ready {
		if err := d.db.QueryRow(
			`UPDATE starlight.starlight.image SET ready=$2 WHERE id=$1 RETURNING id`,
			serial, time.Now().Format(time.RFC3339Nano),
		).Scan(&id); err != nil {
			return err
		}
	} else {
		if err := d.db.QueryRow(
			`UPDATE starlight.starlight.image SET ready=null WHERE id=$1 RETURNING id`,
			serial,
		).Scan(&id); err != nil {
			return err
		}
	}
	return nil
}

func (d *Database) SetImageTag(imageName, tag string, serial int) error {
	txn, err := d.db.Begin()
	if err != nil {
		return err
	}

	if _, err = txn.Exec(`
		INSERT INTO starlight.starlight.tag (image, tag, "imageId") VALUES ($1,$2,$3) 
		ON CONFLICT ON CONSTRAINT "primary"
			DO UPDATE SET  "imageId"=$3`,
		imageName, tag, serial,
	); err != nil {
		return err
	}

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (d *Database) InsertImage(image, hash string, config *v1.ConfigFile, manifest *v1.Manifest, nlayer int) (serial int, existing bool, err error) {
	var (
		ts *time.Time
	)
	if err = d.db.QueryRow(`
			SELECT id, ready FROM starlight.starlight.image
			WHERE image.image=$1 AND image.hash=$2`,
		image, hash).Scan(&serial, &ts); err != nil && err != sql.ErrNoRows {
		return 0, false, err
	}
	if serial != 0 && ts == nil {
		return 0, false, fmt.Errorf("caching is in progress or unfinished, plrease remove: image[%s] digest[%s] serial[%d]", image, hash, serial)
	}
	if serial != 0 && ts != nil {
		return serial, true, nil
	}

	c, err := json.Marshal(config)
	if err != nil {
		return 0, false, err
	}
	m, err := json.Marshal(manifest)
	if err != nil {
		return 0, false, err
	}

	if err = d.db.QueryRow(`
		INSERT INTO starlight.starlight.image(image, hash, config, manifest, ready, nlayer) 
		VALUES ($1, $2, $3, $4, null, $5)
		ON CONFLICT ON CONSTRAINT unique_image_hash DO NOTHING 
		RETURNING id;`,
		image, hash, c, m, nlayer,
	).Scan(&serial); err != nil {
		return 0, false, err
	}

	return serial, false, nil
}

func (d *Database) InsertLayer(
	txn *sql.Tx, size int64, imageSerial, stackIndex int, layerDigest string) (
	fsId int, existing bool, err error) {

	if err = d.db.QueryRow(`
			SELECT id FROM starlight.starlight.filesystem
			WHERE filesystem.digest=$1 AND filesystem.size=$2`,
		layerDigest, size).Scan(&fsId); err != nil && err != sql.ErrNoRows {
		return 0, false, err
	}
	if fsId != 0 {
		existing = true
	}

	if !existing {
		if err = txn.QueryRow(`
				INSERT INTO starlight.starlight.filesystem(digest, size, ready) 
				VALUES ($1, $2, $3)
				RETURNING id;`,
			layerDigest, size, time.Now().Format(time.RFC3339Nano),
		).Scan(&fsId); err != nil {
			return 0, false, err
		}
	}

	// Update layer reference
	var id int
	if err := txn.QueryRow(
		`
		INSERT INTO starlight.starlight.layer(size, image, "stackIndex", layer) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT ON CONSTRAINT unique_image_stack_index
			DO UPDATE SET size=$1, layer=$4
		RETURNING id
		;`,
		size, imageSerial, stackIndex, fsId,
	).Scan(&id); err != nil {
		return 0, false, err
	}

	return fsId, false, nil
}

func (d *Database) InsertFiles(txn *sql.Tx, fsId int, entries map[string]*util.TraceableEntry) (err error) {
	if _, err = txn.Exec(`DELETE FROM starlight.starlight.file WHERE fs=$1`, fsId); err != nil {
		return err
	}

	stmt, err := txn.Prepare(pq.CopyIn("file",
		"hash", "size", "file", "offset", "fs", "metadata",
	))
	if err != nil {
		return err
	}

	for k, v := range entries {
		bMetadata, err := json.Marshal(v)
		if err != nil {
			return err
		}

		if _, err = stmt.Exec(v.Digest, v.Size, k, v.Offset, fsId, string(bMetadata)); err != nil {
			return err
		}
	}

	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	err = stmt.Close()
	if err != nil {
		return err
	}

	return nil
}

func (d *Database) GetImage(image, identifier string) (serial int, err error) {
	var (
		id, nlayer       int64
		config, manifest []byte
	)

	if err = d.db.QueryRow(`
		SELECT id, config, manifest, nlayer FROM starlight.starlight.image
		WHERE ready IS NOT NULL AND image=$1 AND hash=$2 LIMIT 1`,
		image, identifier).Scan(
		&id, &config, &manifest, &nlayer,
	); err != nil && err != sql.ErrNoRows {
		return 0, err
	}

	return
}

func ParseImageReference(ref name.Reference, defaultRegistry string) (imageName, identifier string) {
	imageName = ref.Context().RepositoryStr()
	if ref.Context().RegistryStr() != defaultRegistry {
		imageName = path.Join(ref.Context().RegistryStr(), imageName)
	}
	identifier = ref.Identifier()
	return
}
