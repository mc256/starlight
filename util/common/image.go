/*
   file created by Junlin Chen in 2022

*/

package common

type DeltaImageMetadata struct {
	ManifestSize        int64
	ConfigSize          int64
	StarlightHeaderSize int64
	ContentLength       int64

	Digest          string
	StarlightDigest string
}
