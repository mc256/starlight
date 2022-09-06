/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"database/sql"
	"encoding/json"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/mc256/starlight/util"
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

}

func (d *Database) InsertImage(image, tag, hash string, config *v1.ConfigFile, manifest *v1.Manifest) (int, error) {
	c, err := json.Marshal(config)
	if err != nil {
		return 0, err
	}
	m, err := json.Marshal(manifest)
	if err != nil {
		return 0, err
	}
	var id int
	if err := d.db.QueryRow(
		`
		INSERT INTO starlight.starlight.image(image, tag, hash, config, manifest) 
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT ON CONSTRAINT unique_image_tag
			DO UPDATE SET hash=$3, config=$4, manifest=$5
		RETURNING id
		;`,
		image, tag, hash, c, m,
	).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (d *Database) InsertLayer(size int64, imageSerial, stackIndex int, layerDigest string) (int, error) {
	// Update filesystem table
	var fsId, layerId int
	if err := d.db.QueryRow(
		`
		INSERT INTO starlight.starlight.filesystem(digest, size, updated) 
		VALUES ($1, $2, $3)
		ON CONFLICT ON CONSTRAINT layer_digest_size
			DO UPDATE SET updated=$3
		RETURNING id
		;`,
		layerDigest, size, time.Now().Format(time.RFC3339Nano),
	).Scan(&fsId); err != nil {
		return 0, err
	}

	// Update layer reference
	if err := d.db.QueryRow(
		`
		INSERT INTO starlight.starlight.layer(size, image, "stackIndex", layer) 
		VALUES ($1, $2, $3, $4)
		ON CONFLICT ON CONSTRAINT unique_image_stack_index
			DO UPDATE SET size=$1, layer=$4
		RETURNING id
		;`,
		size, imageSerial, stackIndex, fsId,
	).Scan(&layerId); err != nil {
		return 0, err
	}

	return fsId, nil
}

func (d *Database) InsertFiles(fsId int, entries map[string]*util.TraceableEntry) error {
	txn, err := d.db.Begin()
	if err != nil {
		return err
	}

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

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}
