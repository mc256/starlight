/*
   file created by Junlin Chen in 2022

*/

package proxy

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/containerd/containerd/log"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/lib/pq"
	_ "github.com/lib/pq"
	"github.com/mc256/starlight/client/fs"
	"github.com/mc256/starlight/util/common"
	"github.com/mc256/starlight/util/send"
	"github.com/pkg/errors"
	"math"
	"path"
	"time"
)

type Database struct {
	db *sql.DB
}

func (d *Database) Close() {
	_ = d.db.Close()
}

func NewDatabase(ctx context.Context, conStr string) (*Database, error) {
	var (
		d   *sql.DB
		err error
	)
	i := 0
	for {
		i += 1
		d, err = sql.Open("postgres", conStr)
		if err == nil {
			return &Database{db: d}, nil
		} else if err != nil {
			if i > 10 {
				return nil, err
			}
			log.G(ctx).
				WithError(err).
				Errorf("failed to connect to database, retrying in 5 seconds (%d/10)", i)
			time.Sleep(5 * time.Second)
		}
	}
}

func (d *Database) InitDatabase() error {
	if _, err := d.db.Exec(`
		create table if not exists image
		(
			id       serial,
			image    varchar not null,
			hash     varchar not null,
			config   json,
			manifest json,
			ready    timestamp with time zone,
			nlayer   integer not null,
			primary key (id),
			constraint unique_image_hash
				unique (image, hash)
		);
		
		comment on column image.nlayer is 'number of the non-empty layers';
		
		create table if not exists layer
		(
			id           serial,
			size         bigint,
			image        integer not null,
			"stackIndex" integer,
			layer        integer not null,
			primary key (id),
			constraint unique_image_stack_index
				unique (image, "stackIndex"),
			foreign key (image) references image
				on delete cascade
		);
		
		create table if not exists filesystem
		(
			id     serial,
			digest varchar not null,
			size   bigint,
			ready  timestamp with time zone,
			primary key (id),
			constraint layer_digest_size
				unique (digest, size)
		);
		
		comment on table filesystem is 'Each row represents a filesystem layer where (hash, size) is unique. Each file references back to the id column of this table.';

		
		create table if not exists file
		(
			id       bigserial,
			hash     varchar,
			size     bigint,
			file     varchar,
			"offset" bigint,
			fs       integer,
			"order"  integer[],
			metadata json,
			primary key (id),
			constraint unique_file
				unique (file, fs),
			constraint filesystem_fk
				foreign key (fs) references filesystem
					on delete cascade
		);
		
		create index if not exists fki_filesystem_fk
			on file (fs);
		
		create index if not exists deduplicate_index
			on file (hash, size);
		
		create table if not exists tag
		(
			name      varchar not null,
			tag       varchar not null,
			platform  varchar not null,
			"imageId" bigint  not null,
			constraint "primary"
				primary key (name, tag, platform)
		);

	`); err != nil {
		return err
	}
	return nil
}

func (d *Database) SetImageReady(ready bool, serial int64) error {
	var id int64
	if ready {
		if err := d.db.QueryRow(
			`UPDATE image SET ready=$2 WHERE id=$1 RETURNING id`,
			serial, time.Now().Format(time.RFC3339Nano),
		).Scan(&id); err != nil {
			return err
		}
	} else {
		if err := d.db.QueryRow(
			`UPDATE image SET ready=null WHERE id=$1 RETURNING id`,
			serial,
		).Scan(&id); err != nil {
			return err
		}
	}
	return nil
}

func (d *Database) SetImageTag(name, tag, platform string, serial int64) error {
	txn, err := d.db.Begin()
	if err != nil {
		return err
	}

	if _, err = txn.Exec(`
		INSERT INTO tag (name, tag, platform, "imageId") VALUES ($1,$2,$3,$4) 
		ON CONFLICT ON CONSTRAINT "primary"
			DO UPDATE SET  "imageId"=$4`,
		name, tag, platform, serial,
	); err != nil {
		return err
	}

	err = txn.Commit()
	if err != nil {
		return err
	}

	return nil
}

func (d *Database) InsertImage(image, hash string,
	config *v1.ConfigFile, manifest *v1.Manifest,
	layerCount int64) (
	serial int64, existing bool,
	err error,
) {
	var (
		ts *time.Time
	)
	if err = d.db.QueryRow(`
			SELECT id, ready FROM image
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
		INSERT INTO image(image, hash, config, manifest, ready, nlayer) 
		VALUES ($1, $2, $3, $4, null, $5)
		ON CONFLICT ON CONSTRAINT unique_image_hash DO NOTHING 
		RETURNING id;`,
		image, hash, c, m, layerCount,
	).Scan(&serial); err != nil {
		return 0, false, err
	}

	return serial, false, nil
}

func (d *Database) InsertLayer(
	txn *sql.Tx, size int64, imageSerial, stackIndex int64, layerDigest string) (
	fsId int64, existing bool, err error) {

	if err = d.db.QueryRow(`
			SELECT id FROM filesystem
			WHERE filesystem.digest=$1 AND filesystem.size=$2`,
		layerDigest, size).Scan(&fsId); err != nil && err != sql.ErrNoRows {
		return 0, false, err
	}
	if fsId != 0 {
		existing = true
	}

	if !existing {
		if err = txn.QueryRow(`
				INSERT INTO filesystem(digest, size, ready) 
				VALUES ($1, $2, $3)
				RETURNING id;`,
			layerDigest, size, time.Now().Format(time.RFC3339Nano),
		).Scan(&fsId); err != nil {
			return 0, false, err
		}
	}

	// Update layer reference
	var id int64
	if err := txn.QueryRow(
		`
		INSERT INTO layer(size, image, "stackIndex", layer) 
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

func (d *Database) InsertFiles(txn *sql.Tx, fsId int64, entries map[string]*common.TraceableEntry) (err error) {
	if _, err = txn.Exec(`DELETE FROM file WHERE fs=$1`, fsId); err != nil {
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

func (d *Database) GetImage(image, identifier, platform string) (serial int64, err error) {
	if err = d.db.QueryRow(`
		SELECT "imageId" FROM tag
		WHERE name=$1 AND tag=$2 AND platform=$3 LIMIT 1`,
		image, identifier, platform).Scan(&serial); err != nil && err != sql.ErrNoRows {
		return 0, err
	} else if err == nil {
		return serial, nil
	}

	if err = d.db.QueryRow(`
		SELECT id FROM image
		WHERE ready IS NOT NULL AND image=$1 AND hash=$2 LIMIT 1`,
		image, identifier).Scan(&serial); err != nil {
		return 0, err
	}
	return serial, nil
}

func (d *Database) GetImageByDigest(image, digest string) (serial int64, err error) {
	if err = d.db.QueryRow(`
		SELECT id FROM image
		WHERE ready IS NOT NULL AND image=$1 AND hash=$2 LIMIT 1`,
		image, digest).Scan(&serial); err != nil {
		return 0, err
	}
	return serial, nil
}

func (d *Database) GetManifestAndConfig(serial int64) (config, manifest []byte, digest string, err error) {
	if err = d.db.QueryRow(`
		SELECT config, manifest, hash FROM image
		WHERE id=$1 LIMIT 1`,
		serial).Scan(&config, &manifest, &digest); err != nil {
		return nil, nil, "", err
	}
	return config, manifest, digest, nil
}

func (d *Database) GetLayers(imageSerial int64) ([]*send.ImageLayer, error) {
	rows, err := d.db.Query(`
		SELECT FIS."id", "stackIndex", "digest", FIS."size"
		FROM layer AS L
		LEFT JOIN filesystem AS FIS ON FIS.id = L.layer
		WHERE image=$1
		ORDER BY "stackIndex"`, imageSerial)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	r := make([]*send.ImageLayer, 0)
	for rows.Next() {
		layer := &send.ImageLayer{}
		if err := rows.Scan(&layer.Serial, &layer.StackIndex, &layer.Hash, &layer.UncompressedSize); err != nil {
			return nil, errors.Wrapf(err, "failed to scan file")
		}
		r = append(r, layer)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "failed to load layers")
	}
	return r, nil
}

// GetRoughDeduplicatedLayers returns the likely unique files
// because it would be hard for the database to apply overlayfs correctly, so this deduplication
// does not consider whiteout files.
func (d *Database) GetRoughDeduplicatedLayers(fromSerial, toSerial int64) ([]*send.ImageLayer, error) {
	rows, err := d.db.Query(`
		WITH
		ALPHA AS (
			SELECT 
				L."stackIndex", 
				FI.file, FI.hash, FI.size, FI.metadata 
			FROM layer AS L
			LEFT JOIN filesystem AS FIS ON FIS.id = L.layer
			RIGHT JOIN file AS FI ON FI.fs = FIS.id
			WHERE FI.hash!='' AND image=$1
		),
		BETA AS (
			SELECT 
				L."stackIndex", 
			    FIS."digest" as "filesystemDigest", FIS."size" as "filesystemSize",
				FI.file, FI.hash, FI.size, FI.metadata, FI.id
			FROM layer AS L
			LEFT JOIN filesystem AS FIS ON FIS.id = L.layer
			RIGHT JOIN file AS FI ON FI.fs = FIS.id
			WHERE FI.hash!='' AND image=$2
		),
		ALPHA_UNIQUE AS (
			SELECT MIN("stackIndex") as "minLayer", "hash", "size" FROM ALPHA GROUP BY "hash", "size"
		),
		BETA_UNIQUE AS (
			SELECT MIN("stackIndex") as "minLayer", "hash", "size" FROM BETA GROUP BY "hash", "size"
		),
		BETA_SOURCE AS (
			SELECT * FROM BETA WHERE ("stackIndex", "hash", "size") in (
				SELECT "minLayer", "hash", "size" FROM BETA_UNIQUE
			)
		)
		SELECT DISTINCT "stackIndex", "filesystemDigest", "filesystemSize" from BETA_SOURCE
		WHERE (BETA_SOURCE.hash, BETA_SOURCE.size) not in (
			SELECT ALPHA_UNIQUE.hash, ALPHA_UNIQUE.size 
			FROM ALPHA_UNIQUE
		);`, fromSerial, toSerial)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	r := make([]*send.ImageLayer, 0)
	for rows.Next() {
		layer := &send.ImageLayer{}
		if err := rows.Scan(&layer.StackIndex, &layer.Hash, &layer.UncompressedSize); err != nil {
			return nil, errors.Wrapf(err, "failed to scan file")
		}
		r = append(r, layer)
	}
	if err = rows.Err(); err != nil {
		return nil, errors.Wrapf(err, "failed to load layers")
	}
	return r, nil
}

func (d *Database) GetUniqueFiles(layers []*send.ImageLayer) ([]*send.File, error) {
	lids := make([]int64, 0, len(layers))
	for _, v := range layers {
		lids = append(lids, v.Serial)
	}
	rows, err := d.db.Query(`
		SELECT 
			FI.fs, FI.metadata
		FROM file AS FI
		WHERE FI.fs = ANY($1) AND FI.hash != ''`, pq.Array(lids))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fl := make([]*send.File, 0)
	for rows.Next() {
		var (
			fsId     int64
			metadata []byte
			toc      common.TOCEntry
		)
		if err = rows.Scan(&fsId, &metadata); err != nil {
			return nil, errors.Wrapf(err, "failed to scan file")
		}
		if err = json.Unmarshal(metadata, &toc); err != nil {
			return nil, errors.Wrapf(err, "failed to parse ToC Entry")
		}
		fl = append(fl, &send.File{
			TOCEntry: toc, // no need to parse chunks from the database
			FsId:     fsId,
		})
	}
	return fl, nil
}

func (d *Database) UpdateFileRanks(collection *fs.TraceCollection) (fs []int64, err error) {
	var layersMap []int64

	for _, img := range collection.Groups {
		// get image serial id
		i := img.Images[0]
		var imageSerial, nlayer int64
		if err = d.db.QueryRow(`
			SELECT id, nlayer FROM image
			WHERE ready IS NOT NULL AND hash=$1 LIMIT 1`,
			i).Scan(&imageSerial, &nlayer); err != nil {
			return nil, err
		}

		// get layers of the image
		var rows *sql.Rows
		rows, err = d.db.Query(`
				SELECT 
					L.layer
				FROM layer AS L
				WHERE L.image=$1
				ORDER BY "stackIndex" ASC`, imageSerial)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		layersMap = make([]int64, nlayer)
		idx := 0
		for rows.Next() {
			if err = rows.Scan(&layersMap[idx]); err != nil {
				return nil, err
			}
			idx += 1
		}

		stmt, _ := d.db.Prepare(`
			UPDATE file SET "order" = array_append("order",$1) WHERE fs=$2 and file=$3
		`)
		for _, f := range img.History {
			_, err = stmt.Exec(f.Rank, layersMap[f.Stack], f.FileName)
			if err != nil {
				return nil, err
			}
		}
	}
	return layersMap, nil
}

func (d *Database) GetFilesWithRanks(imageSerial int64) ([]*send.RankedFile, error) {
	rows, err := d.db.Query(`
		SELECT 
			L."stackIndex", 
		    FIS.id,
			(SELECT AVG(o) FROM UNNEST("order") o) as "avgRank", 
			FI.metadata
		FROM layer AS L
		LEFT JOIN filesystem AS FIS ON FIS.id = L.layer
		RIGHT JOIN file AS FI ON FI.fs = FIS.id
		WHERE image=$1
		ORDER BY L."stackIndex" ASC`, imageSerial)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fl := make([]*send.RankedFile, 0)
	for rows.Next() {
		var (
			stackIndex, fsId int64
			rank             sql.NullFloat64
			metadata         []byte
			file             send.File
		)
		if err = rows.Scan(&stackIndex, &fsId, &rank, &metadata); err != nil {
			return nil, errors.Wrapf(err, "failed to scan file")
		}
		if err = json.Unmarshal(metadata, &file); err != nil {
			return nil, errors.Wrapf(err, "failed to parse ToC Entry")
		}
		file.FsId = fsId

		if rank.Valid {
			fl = append(fl, &send.RankedFile{
				File:  file,
				Stack: stackIndex,
				Rank:  rank.Float64,
			})
		} else {
			fl = append(fl, &send.RankedFile{
				File:  file,
				Stack: stackIndex,
				Rank:  math.MaxFloat64,
			})
		}
	}

	return fl, nil
}

func ParseImageReference(ref name.Reference, defaultRegistry string) (imageName, identifier string) {
	imageName = ref.Context().RepositoryStr()
	if ref.Context().RegistryStr() != defaultRegistry {
		imageName = path.Join(ref.Context().RegistryStr(), imageName)
	}
	identifier = ref.Identifier()
	return
}
