package mediametadata

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/cabbagekobe/tunetag/id3v2"
	"github.com/kyangconn/music-online-go/internal/domain"
)

func TestValidateVorbisCommentBlockRejectsAllocationBomb(t *testing.T) {
	body := make([]byte, 8)
	// Empty vendor followed by a count that the upstream parser would use as
	// make(..., 0, count) before discovering that no comments are present.
	binary.LittleEndian.PutUint32(body[4:8], maxVorbisCommentCount+1)
	if err := validateVorbisCommentBlock(body, maxEmbeddedTagBytes); err == nil {
		t.Fatal("oversized Vorbis comment count should be rejected before parsing")
	}
}

func TestValidateVorbisCommentBlockAcceptsBoundedMetadata(t *testing.T) {
	const vendorText = "music-online-test"
	const commentText = "TITLE=Safe"
	vendor := []byte(vendorText)
	comment := []byte(commentText)
	body := make([]byte, 4+len(vendor)+4+4+len(comment))
	binary.LittleEndian.PutUint32(body[:4], uint32(len(vendorText)))
	copy(body[4:], vendor)
	offset := 4 + len(vendor)
	binary.LittleEndian.PutUint32(body[offset:offset+4], 1)
	offset += 4
	binary.LittleEndian.PutUint32(body[offset:offset+4], uint32(len(commentText)))
	copy(body[offset+4:], comment)
	if err := validateVorbisCommentBlock(body, maxEmbeddedTagBytes); err != nil {
		t.Fatalf("bounded Vorbis comments should be accepted: %v", err)
	}
}

func TestValidateVorbisCommentBlockRejectsOversizedPicture(t *testing.T) {
	const pictureSize = 4 + 4 + 4 + 16 + 4 + 5
	const picturePrefix = "METADATA_BLOCK_PICTURE="
	const commentLength = len(picturePrefix) + ((pictureSize+2)/3)*4
	picture := make([]byte, pictureSize)
	// Picture type, empty MIME/description, dimensions, then five bytes of data.
	binary.BigEndian.PutUint32(picture[0:4], uint32(3))
	binary.BigEndian.PutUint32(picture[len(picture)-9:len(picture)-5], uint32(5))
	copy(picture[len(picture)-5:], []byte("cover"))
	comment := []byte(picturePrefix + base64.StdEncoding.EncodeToString(picture))
	body := make([]byte, 8+4+len(comment))
	binary.LittleEndian.PutUint32(body[4:8], 1)
	binary.LittleEndian.PutUint32(body[8:12], uint32(commentLength))
	copy(body[12:], comment)
	if err := validateVorbisCommentBlockWithCover(body, maxEmbeddedTagBytes, 4); err == nil {
		t.Fatal("picture data beyond the configured cover limit should be rejected before decoding by tunetag")
	}
}

func TestValidateAPEv2BoundsChecksFooterBeforeID3v1(t *testing.T) {
	content := make([]byte, 32+128)
	copy(content[:8], "APETAGEX")
	binary.LittleEndian.PutUint32(content[12:16], uint32(maxEmbeddedTagBytes+1))
	copy(content[32:35], "TAG")
	path := filepath.Join(t.TempDir(), "trailing-id3.ape")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatalf("write hostile APE tag: %v", err)
	}
	// #nosec G304 -- path is created inside this test's private temporary directory.
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open hostile APE tag: %v", err)
	}
	defer func() { _ = file.Close() }()
	if err := validateAPEv2Bounds(file, int64(len(content)), maxEmbeddedTagBytes); err == nil {
		t.Fatal("APEv2 footer before an ID3v1 trailer must still honor the tag size limit")
	}
}

func TestReadScannedAudioMetadataPreservesPicardIdentifiers(t *testing.T) {
	recordingID := "123e4567-e89b-42d3-a456-426614174000"
	trackID := "123e4567-e89b-42d3-a456-426614174001"
	releaseID := "123e4567-e89b-42d3-a456-426614174002"
	releaseGroupID := "123e4567-e89b-42d3-a456-426614174003"
	artistIDs := []string{
		"123e4567-e89b-42d3-a456-426614174004",
		"123e4567-e89b-42d3-a456-426614174005",
	}
	tag := &id3v2.Tag{Version: id3v2.V24, Frames: []id3v2.Frame{
		&id3v2.TextFrame{FrameID: "TIT2", Text: []string{"Tagged Track"}},
		&id3v2.TextFrame{FrameID: "TPE1", Text: []string{"Main Artist", "Guest"}},
		&id3v2.TextFrame{FrameID: "TALB", Text: []string{"Release"}},
		&id3v2.TextFrame{FrameID: "TCON", Text: []string{"Ambient", "Electronic"}},
		&id3v2.TextFrame{FrameID: "TDRC", Text: []string{"2024-03-02T12:00:00"}},
		&id3v2.TextFrame{FrameID: "TSRC", Text: []string{"US-ABC-24-12345"}},
		&id3v2.TextFrame{FrameID: "TLEN", Text: []string{"201000"}},
		&id3v2.UserTextFrame{Description: "MusicBrainz Track Id", Value: recordingID},
		&id3v2.UserTextFrame{Description: "MusicBrainz Release Track Id", Value: trackID},
		&id3v2.UserTextFrame{Description: "MusicBrainz Album Id", Value: releaseID},
		&id3v2.UserTextFrame{Description: "MusicBrainz Release Group Id", Value: releaseGroupID},
		&id3v2.UserTextFrame{Description: "MusicBrainz Artist Id", Value: artistIDs[0] + "/" + artistIDs[1]},
	}}
	var content bytes.Buffer
	if err := tag.Encode(&content); err != nil {
		t.Fatalf("encode ID3 tag: %v", err)
	}
	content.Write([]byte{0xff, 0xfb, 0x90, 0x64})
	path := filepath.Join(t.TempDir(), "tagged.mp3")
	if err := os.WriteFile(path, content.Bytes(), 0600); err != nil {
		t.Fatalf("write tagged MP3: %v", err)
	}

	metadata, _, err := Read(path, maxEmbeddedTagBytes, 10*1024*1024)
	if err != nil {
		t.Fatalf("read tagged MP3: %v", err)
	}
	if metadata.Title != "Tagged Track" || metadata.Artist != "Main Artist" || metadata.ReleaseDate != "2024-03-02" ||
		metadata.Duration != 201 || metadata.MusicBrainzRecordingID != recordingID || metadata.MusicBrainzTrackID != trackID ||
		metadata.MusicBrainzReleaseID != releaseID || metadata.MusicBrainzReleaseGroupID != releaseGroupID {
		t.Fatalf("canonical scan metadata was not preserved: %+v", metadata)
	}
	if !reflect.DeepEqual(metadata.Artists, domain.StringList{"Main Artist", "Guest"}) ||
		!reflect.DeepEqual(metadata.Genres, domain.StringList{"Ambient", "Electronic"}) ||
		!reflect.DeepEqual(metadata.ISRCs, domain.StringList{"USABC2412345"}) ||
		!reflect.DeepEqual(metadata.MusicBrainzArtistIDs, domain.StringList(artistIDs)) {
		t.Fatalf("multi-value scan metadata was not preserved: %+v", metadata)
	}
}
