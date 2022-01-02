/*
   Copyright The starlight Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.

   file created by maverick in 2021
*/

package util

import (
	"archive/tar"
	"fmt"
	"math"
	"os"
	"path"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
)

const (

	// TOCTarName is the name of the JSON file in the tar archive in the
	// table of contents gzip stream.
	TOCTarName = "stargz.index.json"

	// FooterSize is the number of bytes in the footer
	//
	// The footer is an empty gzip stream with no compression and an Extra
	// header of the form "%016xSTARGZ", where the 64 bit hex-encoded
	// number is the offset to the gzip stream of JSON TOC.
	//
	// 51 comes from:
	//
	// 10 bytes  gzip header
	// 2  bytes  XLEN (length of Extra field) = 26 (4 bytes header + 16 hex digits + len("STARGZ"))
	// 2  bytes  Extra: SI1 = 'S', SI2 = 'G'
	// 2  bytes  Extra: LEN = 22 (16 hex digits + len("STARGZ"))
	// 22 bytes  Extra: subfield = fmt.Sprintf("%016xSTARGZ", offsetOfTOC)
	// 5  bytes  flate header
	// 8  bytes  gzip footer
	// (End of the eStargz blob)
	//
	// NOTE: For Extra fields, subfield IDs SI1='S' SI2='G' is used for eStargz.
	FooterSize = 51

	// legacyFooterSize is the number of bytes in the legacy stargz footer.
	//
	// 47 comes from:
	//
	//   10 byte gzip header +
	//   2 byte (LE16) length of extra, encoding 22 (16 hex digits + len("STARGZ")) == "\x16\x00" +
	//   22 bytes of extra (fmt.Sprintf("%016xSTARGZ", tocGzipOffset))
	//   5 byte flate header
	//   8 byte gzip footer (two little endian uint32s: digest, size)
	legacyFooterSize = 47

	// TOCJSONDigestAnnotation is an annotation for an image layer. This stores the
	// digest of the TOC JSON.
	// This annotation is valid only when it is specified in `.[]layers.annotations`
	// of an image manifest.
	//
	// This is not needed in Starlight
	//
	// TOCJSONDigestAnnotation = "containerd.io/snapshot/stargz/toc.digest"

	// PrefetchLandmark is a file entry which indicates the end position of
	// prefetch in the stargz file.
	PrefetchLandmark = ".prefetch.landmark"

	// NoPrefetchLandmark is a file entry which indicates that no prefetch should
	// occur in the stargz file.
	NoPrefetchLandmark = ".no.prefetch.landmark"

	EmptyFileHash = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

// jtoc is the JSON-serialized table of contents index of the files in the stargz file.
type jtoc struct {
	Version int         `json:"version"`
	Entries []*TOCEntry `json:"entries"`
}

type OptimizedTraceableEntries struct {
	List    []*OptimizedTraceableEntry
	Ranking float64
}

type ByRanking []*OptimizedTraceableEntries

func (br ByRanking) Len() int {
	return len(br)
}

func (br ByRanking) Less(i, j int) bool {
	nani := math.IsNaN(br[i].Ranking)
	nanj := math.IsNaN(br[j].Ranking)
	if nani && nanj {
		return false
	}
	if nani {
		return false
	}
	if nanj {
		return true
	}
	return br[i].Ranking < br[j].Ranking
}

func (br ByRanking) Swap(i, j int) {
	br[i], br[j] = br[j], br[i]
}

////////////////////////////////////////////////////
type OptimizedTraceableEntry struct {
	*TraceableEntry

	// ------------------------------------
	// SourceImage starts from 1 not 0.
	// index 0 and -1 are reserved for special purpose.
	SourceImage int `json:"si,omitempty"`

	// ------------------------------------
	// Statics

	// AccessCount records number of access during start up
	AccessCount int `json:"ac,omitempty"`
	// SumRank
	SumRank int `json:"sr,omitempty"`
	// SumSquaredRank
	SumSquaredRank float64 `json:"sr2,omitempty"`

	// -------------------------------------
	// Statics populated after initialization
	//ranking float32
	//ranking99 float32
}

func (ote *OptimizedTraceableEntry) Key() string {
	return fmt.Sprintf("%d|%s", ote.SourceImage, ote.Name)
}

func (ote *OptimizedTraceableEntry) AddRanking(ranking int) {
	ote.AccessCount++
	ote.SumRank += ranking
	ote.SumSquaredRank += float64(ranking * ranking)
}

func (ote *OptimizedTraceableEntry) ComputeRank() float64 {
	// average
	ranking := float64(ote.SumRank) / float64(ote.AccessCount)
	//ex := float64(ote.SumRank)/float64(ote.AccessCount)
	//ote.ranking99 = float64(
	//	-3.0*math.Sqrt(ote.SumSquaredRank/float64(ote.AccessCount)- ex * ex)
	//	+ float64(ote.ranking))
	return ranking
}

type ByHashSize []*OptimizedTraceableEntry

func (bhs ByHashSize) Len() int {
	return len(bhs)
}

func (bhs ByHashSize) Less(i, j int) bool {
	if bhs[i].Digest != bhs[j].Digest {
		return bhs[i].Digest < bhs[j].Digest
	}
	return bhs[i].Size < bhs[j].Size
}

func (bhs ByHashSize) Swap(i, j int) {
	bhs[i], bhs[j] = bhs[j], bhs[i]
}

type ByFilename []*OptimizedTraceableEntry

func (bfn ByFilename) Len() int {
	return len(bfn)
}

func (bfn ByFilename) Less(i, j int) bool {
	if bfn[i].Name != bfn[j].Name {
		return bfn[i].Name < bfn[j].Name
	}
	return bfn[i].SourceImage < bfn[j].SourceImage
}

func (bfn ByFilename) Swap(i, j int) {
	bfn[i], bfn[j] = bfn[j], bfn[i]
}

func CompareByFilename(a, b *OptimizedTraceableEntry) int {
	if a.Name != b.Name {
		if a.Name < b.Name {
			return 1
		} else {
			return -1
		}
	} else if a.SourceImage != b.SourceImage {
		if a.SourceImage < b.SourceImage {
			return 1
		} else {
			return -1
		}
	} else {
		return 0
	}
}

////////////////////////////////////////////////////
type TraceableEntry struct {
	*TOCEntry

	Landmark int `json:"lm,omitempty"`

	// We need this otherwise source layer wont show in the toc json

	// Source starts from 1 not 0.
	// index 0 and -1 are reserved for special purpose.
	Source int `json:"s,omitempty"`

	// ConsolidatedSource starts from 1 not 0.
	// index 0 and -1 are reserved for special purpose.
	ConsolidatedSource int `json:"cs,omitempty"`

	Chunks      []*TOCEntry `json:"chunks,omitempty"`
	DeltaOffset *[]int64    `json:"df,omitempty"`

	// UpdateMeta indicates whether this entry is just a metadata update.
	// The content of the file is the same as the old one in the same layer
	// (referring to the same image)
	// If false, it means the content of the file has changed.
	UpdateMeta int `json:"md,omitempty"`
}

func GetRootNode() *TraceableEntry {
	return &TraceableEntry{
		TOCEntry: &TOCEntry{
			Name:    ".",
			Type:    "dir",
			Mode:    0755,
			NumLink: 1,
		},
		Chunks: nil,
	}
}

//  DeepCopy creates a deep copy of the object and clears the source layer identifier
//  You must assign a new source layer
func (t *TraceableEntry) DeepCopy() (d *TraceableEntry) {
	d = &TraceableEntry{
		TOCEntry: t.CopyEntry(),
		Landmark: t.Landmark,
		Source:   t.TOCEntry.GetSourceLayer(),
		Chunks:   t.Chunks,
	}
	return
}

//  ExtendEntry creates a deep copy of the t object and clears the source layer identifier
//  You must assign a new source layer
func ExtendEntry(t *TOCEntry) (d *TraceableEntry) {
	d = &TraceableEntry{
		TOCEntry: t.CopyEntry(),
		Landmark: 0,
		Source:   t.GetSourceLayer(),
		Chunks:   nil,
	}
	return
}

func (t *TraceableEntry) ShiftSource(offset int) {
	t.SetSourceLayer(t.TOCEntry.GetSourceLayer() + offset)
}

// SetSourceLayer sets the index of source layer. index should always starts from 1 if
// the entry comes from an actual layer.
func (t *TraceableEntry) SetSourceLayer(d int) {
	t.TOCEntry.SetSourceLayer(d)
	t.Source = d
}

// GetSourceLayer get the source layer from the entry.
// WARNING: if you get load this object from a JSON serialized source (e.g. database), it might
// not give you the correct information because TOCEntry.sourceLayer is not exported. Please
// use TraceableEntry.Source instead
func (t *TraceableEntry) GetSourceLayer() int {
	t.Source = t.TOCEntry.GetSourceLayer()
	return t.TOCEntry.GetSourceLayer()
}

// SetDeltaOffset sets the offset in the image body.
// If offset is zero, it means no changes were made to the file and the client will do nothing.
func (t *TraceableEntry) SetDeltaOffset(offsets *[]int64) {
	t.DeltaOffset = offsets
}

type TraceableBlobDigest struct {
	digest.Digest `json:"hash"`
	ImageName     string `json:"img"`
}

func (d TraceableBlobDigest) String() string {
	return fmt.Sprintf("%s-%s", d.ImageName, d.Digest.String())
}

// TOCEntry is an entry in the stargz file's TOC (Table of Contents).
type TOCEntry struct {
	// Name is the tar entry's name. It is the complete path
	// stored in the tar file, not just the base name.
	Name string `json:"name"`

	// Type is one of "dir", "reg", "symlink", "hardlink", "char",
	// "block", "fifo", or "chunk".
	// The "chunk" type is used for regular file data chunks past the first
	// TOCEntry; the 2nd chunk and on have only Type ("chunk"), Offset,
	// ChunkOffset, and ChunkSize populated.
	Type string `json:"type"`

	// Size, for regular files, is the logical size of the file.
	Size int64 `json:"size,omitempty"`

	// ModTime3339 is the modification time of the tar entry. Empty
	// means zero or unknown. Otherwise it's in UTC RFC3339
	// format. Use the ModTime method to access the time.Time value.
	ModTime3339 string `json:"modtime,omitempty"`
	modTime     time.Time

	// LinkName, for symlinks and hardlinks, is the link target.
	LinkName string `json:"linkName,omitempty"`

	// Mode is the permission and mode bits.
	Mode int64 `json:"mode,omitempty"`

	// UID is the user ID of the owner.
	UID int `json:"uid,omitempty"`

	// GID is the group ID of the owner.
	GID int `json:"gid,omitempty"`

	// Uname is the username of the owner.
	//
	// In the serialized JSON, this field may only be present for
	// the first entry with the same UID.
	Uname string `json:"userName,omitempty"`

	// Gname is the group name of the owner.
	//
	// In the serialized JSON, this field may only be present for
	// the first entry with the same GID.
	Gname string `json:"groupName,omitempty"`

	// Offset, for regular files, provides the offset in the
	// stargz file to the file's data bytes. See ChunkOffset and
	// ChunkSize.
	Offset int64 `json:"offset,omitempty"`

	nextOffset int64 // the Offset of the next entry with a non-zero Offset

	// DevMajor is the major device number for "char" and "block" types.
	DevMajor int `json:"devMajor,omitempty"`

	// DevMinor is the major device number for "char" and "block" types.
	DevMinor int `json:"devMinor,omitempty"`

	// NumLink is the number of entry names pointing to this entry.
	// Zero means one name references this entry.
	NumLink int

	// Xattrs are the extended attribute for the entry.
	Xattrs map[string][]byte `json:"xattrs,omitempty"`

	// Digest stores the OCI checksum for regular files payload.
	// It has the form "sha256:abcdef01234....".
	Digest string `json:"digest,omitempty"`

	// ChunkOffset is non-zero if this is a chunk of a large,
	// regular file. If so, the Offset is where the gzip header of
	// ChunkSize bytes at ChunkOffset in Name begin.
	//
	// In serialized form, a "chunkSize" JSON field of zero means
	// that the chunk goes to the end of the file. After reading
	// from the stargz TOC, though, the ChunkSize is initialized
	// to a non-zero file for when Type is either "reg" or
	// "chunk".
	ChunkOffset int64 `json:"chunkOffset,omitempty"`
	ChunkSize   int64 `json:"chunkSize,omitempty"`

	// ChunkDigest stores an OCI digest of the chunk. This must be formed
	// as "sha256:0123abcd...".
	ChunkDigest string `json:"chunkDigest,omitempty"`

	CompressedSize int64 `json:"compressedSize,omitempty"`

	sourceLayer int
	landmark    int

	children map[string]*TOCEntry
}

// ModTime returns the entry's modification time.
func (e *TOCEntry) ModTime() time.Time { return e.modTime }

func (e *TOCEntry) InitModTime() {
	e.modTime, _ = time.Parse(time.RFC3339, e.ModTime3339)
}

// NextOffset returns the position (relative to the start of the
// stargz file) of the next gzip boundary after e.Offset.
func (e *TOCEntry) NextOffset() int64 { return e.nextOffset }

// Stat returns a FileInfo value representing e.
func (e *TOCEntry) Stat() os.FileInfo { return fileInfo{e} }

func (e *TOCEntry) Landmark() int { return e.landmark }

// ForeachChild calls f for each child item. If f returns false, iteration ends.
// If e is not a directory, f is not called.
func (e *TOCEntry) ForeachChild(f func(baseName string, ent *TOCEntry) bool) {
	for name, ent := range e.children {
		if !f(name, ent) {
			return
		}
	}
}

// LookupChild returns the directory e's child by its base name.
func (e *TOCEntry) LookupChild(baseName string) (child *TOCEntry, ok bool) {
	child, ok = e.children[baseName]
	return
}

func (e *TOCEntry) AddChild(baseName string, child *TOCEntry) {
	if e.children == nil {
		e.children = make(map[string]*TOCEntry)
	}
	if child.Type == "dir" {
		e.NumLink++ // Entry ".." in the subdirectory links to this directory
	}
	e.children[baseName] = child
}

func (e *TOCEntry) GetChild(baseName string) (*TOCEntry, bool) {
	if e == nil || e.children == nil {
		return nil, false
	}
	item, okay := e.children[baseName]
	return item, okay
}

func (e *TOCEntry) HasChild(baseName string) (r bool) {
	_, r = e.GetChild(baseName)
	return
}

func (e *TOCEntry) Children() map[string]*TOCEntry {
	return e.children
}

func (e *TOCEntry) RemoveChild(baseName string) {
	if e == nil || e.children == nil {
		return
	}
	item, okay := e.children[baseName]
	if !okay {
		return
	}
	if item.Type == "dir" {
		e.NumLink--
	}
	delete(e.children, baseName)
}

func (e *TOCEntry) RemoveAllChildren() {
	if e == nil || e.children == nil {
		return
	}
	for _, item := range e.children {
		if item.Type == "dir" {
			e.NumLink--
		}
	}
	for k := range e.children {
		delete(e.children, k)
	}
}

// Helper Methods

func (e *TOCEntry) SetSourceLayer(d int) {
	e.sourceLayer = d
}

func (e *TOCEntry) GetSourceLayer() int {
	return e.sourceLayer
}

// IsDataType reports whether TOCEntry is a regular file or chunk (something that
// contains regular file data).
func (e *TOCEntry) IsDataType() bool { return e.Type == "reg" || e.Type == "chunk" }

func (e *TOCEntry) IsDir() bool {
	return e.Type == "dir"
}

func (e *TOCEntry) IsMeta() bool {
	return e.Type == "meta"
}

func (e *TOCEntry) IsLandmark() bool {
	return e.Name == PrefetchLandmark || e.Name == NoPrefetchLandmark
}

func (e *TOCEntry) IsRoot() bool {
	return e.Name == "."
}

func (e *TOCEntry) HasChunk() bool {
	return e.Type == "reg" && e.ChunkSize > 0 && e.ChunkSize < e.Size
}

func (e *TOCEntry) IsWhiteoutFile() bool {
	return strings.HasPrefix(path.Base(e.Name), ".wh.")
}

// Other Operations

func (e *TOCEntry) CopyEntry() (c *TOCEntry) {
	c = &TOCEntry{
		Name:           e.Name,
		Type:           e.Type,
		Size:           e.Size,
		ModTime3339:    e.ModTime3339,
		modTime:        e.modTime,
		LinkName:       e.LinkName,
		Mode:           e.Mode,
		UID:            e.UID,
		GID:            e.GID,
		Uname:          e.Uname,
		Gname:          e.Gname,
		Offset:         e.Offset,
		nextOffset:     e.nextOffset,
		DevMajor:       e.DevMajor,
		DevMinor:       e.DevMinor,
		NumLink:        e.NumLink,
		Xattrs:         e.Xattrs,
		Digest:         e.Digest,
		ChunkOffset:    e.ChunkOffset,
		ChunkSize:      e.ChunkSize,
		CompressedSize: e.CompressedSize,
		sourceLayer:    e.sourceLayer,
		landmark:       e.landmark,
	}
	return
}

func (e *TOCEntry) UpdateMetadataFrom(s *TOCEntry) {
	e.Name = s.Name
	e.Type = s.Type
	e.Size = s.Size
	e.ModTime3339 = s.ModTime3339
	e.modTime = s.modTime

	e.LinkName = s.LinkName
	e.Mode = s.Mode
	e.UID = s.UID
	e.GID = s.GID
	e.Uname = s.Uname
	e.Gname = s.Gname

	e.DevMajor = s.DevMajor
	e.DevMinor = s.DevMinor

	e.NumLink = s.NumLink //TODO: We may need to change this later
	// Ignore Offset, nextOffset

	e.Xattrs = s.Xattrs
	e.Digest = s.Digest
	e.ChunkOffset = s.ChunkOffset
	e.ChunkSize = s.ChunkSize

	if e.landmark > s.landmark {
		e.landmark = s.landmark
	}

	// SourceLayer remains unchanged
}

func (e *TOCEntry) ToTarHeader() (h *tar.Header) {
	h = &tar.Header{Format: tar.FormatUSTAR}

	switch e.Type {
	case "hardlink":
		h.Typeflag = tar.TypeLink
		h.Linkname = e.LinkName
	case "symlink":
		h.Typeflag = tar.TypeSymlink
		h.Linkname = e.LinkName
	case "dir":
		h.Typeflag = tar.TypeDir
	case "reg":
		h.Typeflag = tar.TypeReg
		h.Size = e.Size
	case "char":
		h.Typeflag = tar.TypeChar
		h.Devmajor = int64(e.DevMajor)
		h.Devminor = int64(e.DevMinor)
	case "block":
		h.Typeflag = tar.TypeBlock
		h.Devmajor = int64(e.DevMajor)
		h.Devminor = int64(e.DevMinor)
	case "fifo":
		h.Typeflag = tar.TypeFifo
	case "chunk":
		h.Typeflag = tar.TypeReg

	}

	h.Name = e.Name
	h.Mode = e.Mode
	h.Uid = e.UID
	h.Gid = e.GID
	h.Uname = e.Uname
	h.Gname = e.Gname
	h.ModTime = e.modTime

	if len(e.Xattrs) > 0 {
		for k, v := range e.Xattrs {
			h.PAXRecords["SCHILY.xattr."+k] = string(v)
		}
	}

	return
}

func MakeEmptyFile(fileName string) (e *TOCEntry) {
	e = &TOCEntry{
		Name:    fileName,
		Type:    "reg",
		NumLink: 1,
		Digest:  EmptyFileHash,
	}
	return e
}

// MakeWhiteoutFile parent should include the trailing backslash
func MakeWhiteoutFile(baseName, parentDir string) (e *TOCEntry) {
	e = MakeEmptyFile(path.Join(parentDir, fmt.Sprintf(".wh.%s", baseName)))
	return e
}

func MakeOpaqueWhiteoutFile(parentDir string) (e *TOCEntry) {
	e = MakeEmptyFile(path.Join(parentDir, ".wh..wh..opq"))
	return e
}

func MakeDir(dirName string) (e *TOCEntry) {
	e = &TOCEntry{
		Name:    dirName,
		Type:    "dir",
		Mode:    0755,
		NumLink: 2, // The directory itself(.).
	}
	return
}

/*
TOCEntry section ends.
*/

// fileInfo implements os.FileInfo using the wrapped *TOCEntry.
type fileInfo struct{ e *TOCEntry }

var _ os.FileInfo = fileInfo{}

func (fi fileInfo) Name() string       { return path.Base(fi.e.Name) }
func (fi fileInfo) IsDir() bool        { return fi.e.Type == "dir" }
func (fi fileInfo) Size() int64        { return fi.e.Size }
func (fi fileInfo) ModTime() time.Time { return fi.e.ModTime() }
func (fi fileInfo) Sys() interface{}   { return fi.e }
func (fi fileInfo) Mode() (m os.FileMode) {
	m = os.FileMode(fi.e.Mode) & os.ModePerm
	switch fi.e.Type {
	case "dir":
		m |= os.ModeDir
	case "symlink":
		m |= os.ModeSymlink
	case "char":
		m |= os.ModeDevice | os.ModeCharDevice
	case "block":
		m |= os.ModeDevice
	case "fifo":
		m |= os.ModeNamedPipe
	}
	// TODO: ModeSetuid, ModeSetgid, if/as needed.
	return m
}

// TOCEntryVerifier holds verifiers that are usable for verifying chunks contained
// in a eStargz blob.
type TOCEntryVerifier interface {

	// Verifier provides a content verifier that can be used for verifying the
	// contents of the specified TOCEntry.
	Verifier(ce *TOCEntry) (digest.Verifier, error)
}
