package service

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/cabbagekobe/tunetag"
	"github.com/cabbagekobe/tunetag/aac"
	"github.com/cabbagekobe/tunetag/ape"
	"github.com/cabbagekobe/tunetag/asf"
	"github.com/cabbagekobe/tunetag/flac"
	"github.com/cabbagekobe/tunetag/id3v1"
	"github.com/cabbagekobe/tunetag/id3v2"
	"github.com/cabbagekobe/tunetag/mp4"
	"github.com/cabbagekobe/tunetag/ogg"
	"github.com/kyangconn/music-online-go/internal/domain"
)

const (
	maxEmbeddedTagBytes     = 16 * 1024 * 1024
	maxTextChunkBytes       = 1024 * 1024
	maxTuneTagMetadataBytes = 64 * 1024 * 1024
	maxVorbisCommentCount   = 4096
	maxVorbisCommentBytes   = 16 * 1024 * 1024
	maxFLACMetadataBlocks   = 1024
)

type scannedCover struct {
	MIME string
	Data []byte
	Type tunetag.PictureType
}

func readScannedAudioMetadata(path string, maxTagBytes, maxCoverBytes int64) (*domain.CreateMusicRequest, *scannedCover, error) {
	maxTagBytes = normalizedTagLimit(maxTagBytes)
	ext := strings.ToLower(filepath.Ext(path))
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return &domain.CreateMusicRequest{}, nil, err
	}
	defer func() { _ = file.Close() }()
	format, err := tunetag.Detect(file)
	if err != nil {
		return &domain.CreateMusicRequest{}, nil, err
	}
	if format == tunetag.FormatWAV {
		req, cover, err := readWAVMetadata(file, maxTagBytes)
		return req, boundedScannedCover(cover, maxCoverBytes), err
	}
	if format == tunetag.FormatAIFF {
		req, cover, err := readAIFFMetadata(file, maxTagBytes)
		return req, boundedScannedCover(cover, maxCoverBytes), err
	}
	if format == tunetag.FormatASF {
		return readASFMetadata(file, maxTagBytes, maxCoverBytes)
	}
	if err := ensureTuneTagReadBoundFile(file, format, maxTagBytes, maxCoverBytes); err != nil {
		return &domain.CreateMusicRequest{}, nil, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return &domain.CreateMusicRequest{}, nil, err
	}

	req := &domain.CreateMusicRequest{}
	var cover *scannedCover
	var detailErr error
	switch format {
	case tunetag.FormatID3v1, tunetag.FormatID3v2:
		if ext == ".aac" {
			parsed, err := aac.Read(file)
			if err != nil {
				detailErr = err
			} else {
				applyScalarTag(req, parsed)
				if embedded := applyID3Tag(req, parsed.V2); embedded != nil {
					cover = preferScannedCover(cover, embedded)
				}
			}
			break
		}
		if format == tunetag.FormatID3v2 {
			parsed, err := id3v2.Read(file)
			if err != nil {
				detailErr = err
			} else {
				applyScalarTag(req, parsed)
				if embedded := applyID3Tag(req, parsed); embedded != nil {
					cover = preferScannedCover(cover, embedded)
				}
			}
		}
		// ID3v1 remains a useful fallback for partially tagged MP3 files.
		if parsed, err := id3v1.Read(file); err == nil {
			applyID3v1Fallback(req, parsed)
		} else if format == tunetag.FormatID3v1 {
			detailErr = err
		}
	case tunetag.FormatAAC:
		parsed, err := aac.Read(file)
		if err != nil {
			detailErr = err
		} else {
			applyScalarTag(req, parsed)
			if embedded := applyID3Tag(req, parsed.V2); embedded != nil {
				cover = preferScannedCover(cover, embedded)
			}
		}
	case tunetag.FormatFLAC:
		parsed, err := flac.Read(file)
		if err != nil {
			detailErr = err
		} else {
			applyVorbisComments(req, parsed.VorbisComment())
			cover = bestFLACCover(parsed.Pictures(), maxCoverBytes)
			req.Duration = flacDurationSeconds(parsed)
		}
	case tunetag.FormatOgg:
		parsed, err := ogg.Read(file)
		if err != nil {
			detailErr = err
		} else {
			applyScalarTag(req, parsed)
			applyVorbisComments(req, parsed.Comments)
			cover = bestVorbisCover(parsed.Comments, maxCoverBytes)
			req.Duration = oggDurationSeconds(file)
		}
	case tunetag.FormatMP4:
		// tunetag's MP4 reader currently accepts only a path. The moov box is
		// bounded above and the caller verifies source stability after parsing.
		parsed, err := tunetag.OpenMP4(path)
		if err != nil {
			detailErr = err
		} else {
			applyMP4Tag(req, parsed.Tag)
			cover = bestMP4Cover(parsed.Tag, maxCoverBytes)
			applyMP4Freeform(req, parsed.Tag)
			req.Duration = mp4DurationSeconds(path)
		}
	case tunetag.FormatAPE:
		parsed, err := ape.Read(file)
		if err != nil {
			detailErr = err
		} else {
			applyScalarTag(req, parsed)
			applyAPETags(req, parsed.Items)
			cover = bestAPECover(parsed.Items, maxCoverBytes)
		}
	}

	sanitizeScannedMetadata(req)
	cover = boundedScannedCover(cover, maxCoverBytes)
	return req, cover, detailErr
}

func ensureTuneTagReadBoundFile(file *os.File, format tunetag.Format, maxTagBytes, maxCoverBytes int64) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	var header [10]byte
	if _, err := file.ReadAt(header[:], 0); err == nil && string(header[:3]) == "ID3" {
		if header[6]&0x80 != 0 || header[7]&0x80 != 0 || header[8]&0x80 != 0 || header[9]&0x80 != 0 {
			return errors.New("embedded ID3 tag has an invalid synchsafe size")
		}
		size := int64(header[6]&0x7f)<<21 | int64(header[7]&0x7f)<<14 | int64(header[8]&0x7f)<<7 | int64(header[9]&0x7f)
		if size > maxTagBytes {
			return fmt.Errorf("embedded ID3 tag exceeds %d bytes", maxTagBytes)
		}
		if size+10 > info.Size() {
			return errors.New("embedded ID3 tag exceeds file size")
		}
	}
	if err := validateAPEv2Bounds(file, info.Size(), maxTagBytes); err != nil {
		return err
	}
	if string(header[:4]) == "fLaC" {
		return ensureFLACMetadataBound(file, info.Size(), maxTagBytes, maxCoverBytes)
	}
	if format == tunetag.FormatOgg {
		return ensureOggCommentsBound(file, maxTagBytes, maxCoverBytes)
	}
	if format == tunetag.FormatMP4 {
		_, size, found := findMP4Box(file, 0, info.Size(), "moov")
		if found && size > maxTagBytes {
			return fmt.Errorf("MP4 metadata exceeds %d bytes", maxTagBytes)
		}
	}
	return nil
}

func validateAPEv2Bounds(file *os.File, fileSize, maxTagBytes int64) error {
	if fileSize < 32 {
		return nil
	}
	offsets := []int64{fileSize - 32}
	if fileSize >= id3v1.TagSize+32 {
		var marker [3]byte
		if _, err := file.ReadAt(marker[:], fileSize-id3v1.TagSize); err == nil && string(marker[:]) == "TAG" {
			offsets = append(offsets, fileSize-id3v1.TagSize-32)
		}
	}
	for _, offset := range offsets {
		var footer [32]byte
		if _, err := file.ReadAt(footer[:], offset); err != nil || string(footer[:8]) != "APETAGEX" {
			continue
		}
		size := int64(binary.LittleEndian.Uint32(footer[12:16]))
		itemCount := binary.LittleEndian.Uint32(footer[16:20])
		if size > maxTagBytes {
			return fmt.Errorf("APEv2 tag exceeds %d bytes", maxTagBytes)
		}
		if itemCount > maxVorbisCommentCount {
			return fmt.Errorf("APEv2 item count %d exceeds safe limit", itemCount)
		}
		// The size includes the footer but not the optional header. The
		// body must therefore begin inside the same physical file.
		if size < 32 || offset-int64(size)+32 < 0 {
			return errors.New("APEv2 tag exceeds file bounds")
		}
	}
	return nil
}

func ensureFLACMetadataBound(file *os.File, fileSize, maxTagBytes, maxCoverBytes int64) error {
	var marker [4]byte
	if _, err := file.ReadAt(marker[:], 0); err != nil {
		return err
	}
	if string(marker[:]) != "fLaC" {
		return nil
	}
	position := int64(4)
	total := int64(0)
	blockCount := 0
	for position+4 <= fileSize {
		blockCount++
		if blockCount > maxFLACMetadataBlocks {
			return errors.New("FLAC metadata block count exceeds safe limit")
		}
		var header [4]byte
		if _, err := file.ReadAt(header[:], position); err != nil {
			return err
		}
		size := int64(header[1])<<16 | int64(header[2])<<8 | int64(header[3])
		total += 4 + size
		if total > maxTagBytes {
			return fmt.Errorf("FLAC metadata exceeds %d bytes", maxTagBytes)
		}
		position += 4 + size
		if position > fileSize {
			return errors.New("FLAC metadata exceeds file size")
		}
		blockType := header[0] & 0x7f
		if blockType == 4 || blockType == 6 {
			body := make([]byte, size)
			if _, err := file.ReadAt(body, position-size); err != nil {
				return err
			}
			if blockType == 4 {
				if err := validateVorbisCommentBlock(body, maxTagBytes); err != nil {
					return fmt.Errorf("unsafe FLAC Vorbis comment: %w", err)
				}
			} else if err := validateFLACPictureBlock(body, maxCoverBytes); err != nil {
				return fmt.Errorf("unsafe FLAC picture: %w", err)
			}
		}
		if header[0]&0x80 != 0 {
			return nil
		}
	}
	return errors.New("FLAC metadata is truncated")
}

func normalizedTagLimit(limit int64) int64 {
	if limit <= 0 {
		return maxEmbeddedTagBytes
	}
	if limit > maxTuneTagMetadataBytes {
		return maxTuneTagMetadataBytes
	}
	return limit
}

func validateVorbisCommentBlock(body []byte, maxTagBytes int64) error {
	return validateVorbisCommentBlockWithCover(body, maxTagBytes, 0)
}

func validateVorbisCommentBlockWithCover(body []byte, maxTagBytes, maxCoverBytes int64) error {
	if int64(len(body)) > maxTagBytes || len(body) < 8 {
		return errors.New("comment block has an unsafe size")
	}
	offset := 0
	readLength := func(label string) (uint32, error) {
		if offset+4 > len(body) {
			return 0, fmt.Errorf("%s length is truncated", label)
		}
		value := binary.LittleEndian.Uint32(body[offset : offset+4])
		offset += 4
		return value, nil
	}
	vendorLength, err := readLength("vendor")
	if err != nil {
		return err
	}
	if vendorLength > maxTextChunkBytes || int64(offset)+int64(vendorLength) > int64(len(body)) {
		return errors.New("vendor string exceeds safe bounds")
	}
	offset += int(vendorLength)
	commentCount, err := readLength("comment count")
	if err != nil {
		return err
	}
	// tunetag allocates a slice using this untrusted count. Check it before the
	// dependency sees the bytes so a tiny file cannot request a huge allocation.
	if commentCount > maxVorbisCommentCount {
		return fmt.Errorf("comment count %d exceeds safe limit", commentCount)
	}
	for index := uint32(0); index < commentCount; index++ {
		commentLength, err := readLength("comment")
		if err != nil {
			return err
		}
		if int64(commentLength) > minInt64(maxVorbisCommentBytes, maxTagBytes) || int64(offset)+int64(commentLength) > int64(len(body)) {
			return fmt.Errorf("comment %d exceeds safe bounds", index)
		}
		comment := body[offset : offset+int(commentLength)]
		if err := validateVorbisPictureComment(comment, maxCoverBytes); err != nil {
			return fmt.Errorf("comment %d contains an unsafe picture: %w", index, err)
		}
		offset += int(commentLength)
	}
	return nil
}

func validateVorbisPictureComment(comment []byte, maxCoverBytes int64) error {
	if maxCoverBytes <= 0 {
		return nil
	}
	separator := bytes.IndexByte(comment, '=')
	if separator < 0 || !bytes.EqualFold(bytes.TrimSpace(comment[:separator]), []byte(ogg.FieldMetadataBlockPicture)) {
		return nil
	}
	encoded := bytes.TrimSpace(comment[separator+1:])
	maxBlockBytes := maxCoverBytes + 2*maxTextChunkBytes + 64
	if int64(base64.StdEncoding.DecodedLen(len(encoded))) > maxBlockBytes {
		return errors.New("decoded picture block exceeds safe bounds")
	}
	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(encoded)))
	n, err := base64.StdEncoding.Decode(decoded, encoded)
	if err != nil {
		return errors.New("picture is not valid base64")
	}
	if err := validateFLACPictureBlock(decoded[:n], maxCoverBytes); err != nil {
		return err
	}
	return nil
}

func validateFLACPictureBlock(body []byte, maxCoverBytes int64) error {
	if maxCoverBytes <= 0 {
		maxCoverBytes = 10 * 1024 * 1024
	}
	offset := 0
	readBigEndianLength := func(label string) (uint32, error) {
		if offset+4 > len(body) {
			return 0, fmt.Errorf("%s is truncated", label)
		}
		value := binary.BigEndian.Uint32(body[offset : offset+4])
		offset += 4
		return value, nil
	}
	if _, err := readBigEndianLength("picture type"); err != nil {
		return err
	}
	for _, label := range []string{"MIME type", "description"} {
		length, err := readBigEndianLength(label + " length")
		if err != nil {
			return err
		}
		if length > maxTextChunkBytes || int64(offset)+int64(length) > int64(len(body)) {
			return fmt.Errorf("%s exceeds safe bounds", label)
		}
		offset += int(length)
	}
	// Width, height, depth and colour count are fixed-width integers.
	if offset+16 > len(body) {
		return errors.New("picture dimensions are truncated")
	}
	offset += 16
	dataLength, err := readBigEndianLength("picture data length")
	if err != nil {
		return err
	}
	if int64(dataLength) > maxCoverBytes || int64(offset)+int64(dataLength) > int64(len(body)) {
		return errors.New("picture data exceeds configured cover limit")
	}
	return nil
}

func ensureOggCommentsBound(file *os.File, maxTagBytes, maxCoverBytes int64) error {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	var packet []byte
	var currentSerial uint32
	hasCurrentSerial := false
	var targetSerial uint32
	hasTargetSerial := false
	consumed := int64(0)
	for page := 0; page < maxFLACMetadataBlocks; page++ {
		var header [27]byte
		if _, err := io.ReadFull(file, header[:]); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				if hasTargetSerial {
					return errors.New("ogg comment packet is missing or truncated")
				}
				return nil
			}
			return err
		}
		consumed += int64(len(header))
		if string(header[:4]) != "OggS" || header[4] != 0 {
			return errors.New("invalid Ogg page header")
		}
		serial := binary.LittleEndian.Uint32(header[14:18])
		segmentCount := int(header[26])
		segments := make([]byte, segmentCount)
		if _, err := io.ReadFull(file, segments); err != nil {
			return err
		}
		consumed += int64(segmentCount)
		bodySize := 0
		for _, length := range segments {
			bodySize += int(length)
		}
		body := make([]byte, bodySize)
		if _, err := io.ReadFull(file, body); err != nil {
			return err
		}
		consumed += int64(bodySize)
		if hasTargetSerial && serial != targetSerial {
			continue
		}
		if !hasCurrentSerial || currentSerial != serial {
			packet = packet[:0]
			currentSerial = serial
			hasCurrentSerial = true
		}
		bodyOffset := 0
		for _, segmentLengthByte := range segments {
			segmentLength := int(segmentLengthByte)
			if bodyOffset+segmentLength > len(body) {
				return errors.New("ogg lacing table exceeds page body")
			}
			if int64(len(packet))+int64(segmentLength) > maxTagBytes {
				return fmt.Errorf("ogg metadata packet exceeds %d bytes", maxTagBytes)
			}
			packet = append(packet, body[bodyOffset:bodyOffset+segmentLength]...)
			bodyOffset += segmentLength
			if segmentLengthByte == 255 {
				continue
			}
			switch {
			case bytes.HasPrefix(packet, []byte{0x01, 'v', 'o', 'r', 'b', 'i', 's'}), bytes.HasPrefix(packet, []byte("OpusHead")):
				targetSerial = serial
				hasTargetSerial = true
			case bytes.HasPrefix(packet, []byte{0x03, 'v', 'o', 'r', 'b', 'i', 's'}):
				return validateVorbisCommentBlockWithCover(packet[7:], maxTagBytes, maxCoverBytes)
			case bytes.HasPrefix(packet, []byte("OpusTags")):
				return validateVorbisCommentBlockWithCover(packet[8:], maxTagBytes, maxCoverBytes)
			}
			packet = packet[:0]
		}
		if consumed > maxTagBytes+2*1024*1024 {
			if hasTargetSerial {
				return errors.New("ogg comment packet was not found within safe scan bounds")
			}
			return nil
		}
	}
	return errors.New("ogg metadata page count exceeds safe limit")
}

func boundedScannedCover(cover *scannedCover, maxCoverBytes int64) *scannedCover {
	if cover == nil || len(cover.Data) == 0 {
		return nil
	}
	if maxCoverBytes <= 0 || int64(len(cover.Data)) > maxCoverBytes {
		return nil
	}
	return cover
}

// scannedScalarTag is the common subset exposed by tunetag's concrete readers.
// Keeping it local lets the scanner parse an already-checked file descriptor
// instead of reopening an untrusted network path through tunetag.Open.
type scannedScalarTag interface {
	Title() string
	Artist() string
	AlbumArtist() string
	Album() string
	Year() int
	TrackNumber() (int, int)
	DiscNumber() (int, int)
	Genre() string
	Comment() string
}

func applyScalarTag(req *domain.CreateMusicRequest, tag scannedScalarTag) {
	if tag == nil {
		return
	}
	req.Title = firstNonEmpty(req.Title, tag.Title())
	req.Artist = firstNonEmpty(req.Artist, tag.Artist())
	req.AlbumArtist = firstNonEmpty(req.AlbumArtist, tag.AlbumArtist())
	req.Album = firstNonEmpty(req.Album, tag.Album())
	if req.Year == 0 {
		req.Year = tag.Year()
	}
	if req.TrackNumber == 0 {
		req.TrackNumber, req.TrackTotal = tag.TrackNumber()
	}
	if req.DiscNumber == 0 {
		req.DiscNumber, req.DiscTotal = tag.DiscNumber()
	}
	req.Genre = firstNonEmpty(req.Genre, tag.Genre())
	req.Genres = appendTagValues(req.Genres, tag.Genre())
	req.Comment = firstNonEmpty(req.Comment, tag.Comment())
}

func applyID3v1Fallback(req *domain.CreateMusicRequest, tag *id3v1.Tag) {
	if tag == nil {
		return
	}
	req.Title = firstNonEmpty(req.Title, tag.Title)
	req.Artist = firstNonEmpty(req.Artist, tag.Artist)
	req.Album = firstNonEmpty(req.Album, tag.Album)
	req.Comment = firstNonEmpty(req.Comment, tag.Comment)
	req.Genre = firstNonEmpty(req.Genre, tag.GenreName())
	req.Genres = appendTagValues(req.Genres, tag.GenreName())
	if req.Year == 0 {
		req.Year, _ = strconv.Atoi(strings.TrimSpace(tag.Year))
	}
	if req.TrackNumber == 0 {
		req.TrackNumber = int(tag.Track)
	}
}

func bestFLACCover(pictures []*flac.Picture, maxCoverBytes int64) *scannedCover {
	var best *scannedCover
	for _, picture := range pictures {
		if picture == nil || len(picture.Data) == 0 || maxCoverBytes <= 0 || int64(len(picture.Data)) > maxCoverBytes {
			continue
		}
		pictureType := tunetag.PictureOther
		if picture.PictureType <= uint32(tunetag.PicturePublisherLogo) {
			pictureType = tunetag.PictureType(picture.PictureType)
		}
		candidate := &scannedCover{MIME: picture.MIME, Data: picture.Data, Type: pictureType}
		best = preferScannedCover(best, candidate)
		if pictureType == tunetag.PictureCoverFront {
			break
		}
	}
	return best
}

func bestVorbisCover(comments *flac.VorbisComment, maxCoverBytes int64) *scannedCover {
	if comments == nil || maxCoverBytes <= 0 {
		return nil
	}
	maxBlockBytes := maxCoverBytes + 2*maxTextChunkBytes + 64
	var pictures []*flac.Picture
	for _, value := range comments.Get(ogg.FieldMetadataBlockPicture) {
		value = strings.TrimSpace(value)
		if int64(base64.StdEncoding.DecodedLen(len(value))) > maxBlockBytes {
			continue
		}
		body, err := base64.StdEncoding.DecodeString(value)
		if err != nil || int64(len(body)) > maxBlockBytes || validateFLACPictureBlock(body, maxCoverBytes) != nil {
			continue
		}
		picture, err := flac.ParsePicture(body)
		if err == nil {
			pictures = append(pictures, picture)
		}
	}
	return bestFLACCover(pictures, maxCoverBytes)
}

func bestAPECover(items []ape.Item, maxCoverBytes int64) *scannedCover {
	var best *scannedCover
	for _, item := range items {
		if item.Type != ape.ItemBinary {
			continue
		}
		pictureType := tunetag.PictureOther
		switch {
		case strings.EqualFold(item.Key, ape.KeyCoverArtFront):
			pictureType = tunetag.PictureCoverFront
		case strings.EqualFold(item.Key, ape.KeyCoverArtBack):
			pictureType = tunetag.PictureCoverBack
		case !strings.EqualFold(item.Key, ape.KeyCoverArtOther):
			continue
		}
		separator := bytes.IndexByte(item.Value, 0)
		if separator < 0 || maxCoverBytes <= 0 || int64(len(item.Value)-separator-1) > maxCoverBytes {
			continue
		}
		candidate := &scannedCover{Data: item.Value[separator+1:], Type: pictureType}
		best = preferScannedCover(best, candidate)
		if pictureType == tunetag.PictureCoverFront {
			break
		}
	}
	return best
}

func applyMP4Tag(req *domain.CreateMusicRequest, tag *mp4.Ilst) {
	if tag == nil {
		return
	}
	req.Title = tag.Title()
	req.Artist = tag.Artist()
	req.AlbumArtist = tag.AlbumArtist()
	req.Album = tag.Album()
	req.Year = tag.Year()
	track, trackTotal := tag.Track()
	req.TrackNumber, req.TrackTotal = int(track), int(trackTotal)
	disc, discTotal := tag.Disc()
	req.DiscNumber, req.DiscTotal = int(disc), int(discTotal)
	req.Genre = tag.GenreText()
	req.Genres = appendTagValues(req.Genres, tag.GenreText())
	req.Comment = tag.Comment()
}

func bestMP4Cover(tag *mp4.Ilst, maxCoverBytes int64) *scannedCover {
	if tag == nil {
		return nil
	}
	for _, picture := range tag.Pictures() {
		if picture == nil || maxCoverBytes <= 0 || int64(len(picture.Payload)) > maxCoverBytes {
			continue
		}
		mime := ""
		switch picture.TypeCode {
		case mp4.DataTypeJPEG:
			mime = "image/jpeg"
		case mp4.DataTypePNG:
			mime = "image/png"
		}
		return &scannedCover{MIME: mime, Data: picture.Payload, Type: tunetag.PictureCoverFront}
	}
	return nil
}

func preferScannedCover(current, candidate *scannedCover) *scannedCover {
	if candidate == nil {
		return current
	}
	if current == nil || candidate.Type == tunetag.PictureCoverFront {
		return candidate
	}
	return current
}

func applyID3Tag(req *domain.CreateMusicRequest, tag *id3v2.Tag) *scannedCover {
	if tag == nil {
		return nil
	}
	if req.Title == "" {
		req.Title = tag.Title()
	}
	if req.Artist == "" {
		req.Artist = tag.Artist()
	}
	if req.AlbumArtist == "" {
		req.AlbumArtist = tag.AlbumArtist()
	}
	if req.Album == "" {
		req.Album = tag.Album()
	}
	if req.Genre == "" {
		req.Genre = tag.Genre()
	}
	if req.Comment == "" {
		req.Comment = tag.Comment()
	}
	if req.Year == 0 {
		req.Year = tag.Year()
	}
	if req.TrackNumber == 0 {
		req.TrackNumber, req.TrackTotal = tag.TrackNumber()
	}
	if req.DiscNumber == 0 {
		req.DiscNumber, req.DiscTotal = tag.DiscNumber()
	}

	for _, frame := range tag.Frames {
		switch value := frame.(type) {
		case *id3v2.TextFrame:
			switch value.FrameID {
			case "TPE1":
				req.Artists = appendTagValues(req.Artists, value.Text...)
			case "TPE2":
				req.AlbumArtists = appendTagValues(req.AlbumArtists, value.Text...)
			case "TCON":
				req.Genres = appendTagValues(req.Genres, value.Text...)
			case "TDRC":
				req.ReleaseDate = firstNonEmpty(req.ReleaseDate, value.String())
			case "TDOR":
				req.OriginalReleaseDate = firstNonEmpty(req.OriginalReleaseDate, value.String())
			case "TDRL":
				req.ReleaseDate = firstNonEmpty(req.ReleaseDate, value.String())
			case "TSRC":
				req.ISRCs = appendTagValues(req.ISRCs, value.Text...)
			case "TLEN":
				if milliseconds, err := strconv.ParseInt(strings.TrimSpace(value.String()), 10, 64); err == nil && milliseconds > 0 {
					seconds := milliseconds / 1000
					if milliseconds%1000 >= 500 {
						seconds++
					}
					req.Duration = durationSecondsFromInt64(seconds)
				}
			}
		case *id3v2.UserTextFrame:
			applyNamedMetadataValue(req, value.Description, value.Value)
		case *id3v2.UFIDFrame:
			if strings.EqualFold(strings.TrimSpace(value.Owner), "http://musicbrainz.org") {
				req.MusicBrainzRecordingID = firstNonEmpty(req.MusicBrainzRecordingID, string(value.Identifier))
			}
		}
	}

	var first *scannedCover
	for _, picture := range tag.PictureFrames() {
		if len(picture.Data) == 0 {
			continue
		}
		candidate := &scannedCover{MIME: picture.MIME, Data: picture.Data, Type: tunetag.PictureType(picture.PictureType)}
		if candidate.Type == tunetag.PictureCoverFront {
			return candidate
		}
		if first == nil {
			first = candidate
		}
	}
	return first
}

func applyVorbisComments(req *domain.CreateMusicRequest, comments *flac.VorbisComment) {
	if comments == nil {
		return
	}
	req.Title = firstNonEmpty(req.Title, comments.First("TITLE"))
	req.Artist = firstNonEmpty(req.Artist, comments.First("ARTIST"))
	req.AlbumArtist = firstNonEmpty(req.AlbumArtist, comments.First("ALBUMARTIST"))
	req.Album = firstNonEmpty(req.Album, comments.First("ALBUM"))
	req.Genre = firstNonEmpty(req.Genre, comments.First("GENRE"))
	req.Comment = firstNonEmpty(req.Comment, comments.First("DESCRIPTION"), comments.First("COMMENT"))
	if req.TrackNumber == 0 {
		req.TrackNumber, req.TrackTotal = parseTrackPair(comments.First("TRACKNUMBER"))
		if total, err := strconv.Atoi(strings.TrimSpace(comments.First("TRACKTOTAL"))); err == nil && total > 0 {
			req.TrackTotal = total
		}
	}
	if req.DiscNumber == 0 {
		req.DiscNumber, req.DiscTotal = parseTrackPair(comments.First("DISCNUMBER"))
		if total, err := strconv.Atoi(strings.TrimSpace(comments.First("DISCTOTAL"))); err == nil && total > 0 {
			req.DiscTotal = total
		}
	}
	req.Artists = appendTagValues(req.Artists, comments.Get("ARTIST")...)
	req.Artists = appendTagValues(req.Artists, comments.Get("ARTISTS")...)
	req.AlbumArtists = appendTagValues(req.AlbumArtists, comments.Get("ALBUMARTIST")...)
	req.AlbumArtists = appendTagValues(req.AlbumArtists, comments.Get("ALBUMARTISTS")...)
	req.Genres = appendTagValues(req.Genres, comments.Get("GENRE")...)
	req.ReleaseDate = firstNonEmpty(req.ReleaseDate, comments.First("RELEASEDATE"), comments.First("DATE"))
	req.OriginalReleaseDate = firstNonEmpty(req.OriginalReleaseDate, comments.First("ORIGINALDATE"), comments.First("ORIGINALYEAR"))
	req.ISRCs = appendTagValues(req.ISRCs, comments.Get("ISRC")...)
	req.MusicBrainzRecordingID = firstNonEmpty(req.MusicBrainzRecordingID, comments.First("MUSICBRAINZ_TRACKID"))
	req.MusicBrainzTrackID = firstNonEmpty(req.MusicBrainzTrackID, comments.First("MUSICBRAINZ_RELEASETRACKID"))
	req.MusicBrainzReleaseID = firstNonEmpty(req.MusicBrainzReleaseID, comments.First("MUSICBRAINZ_ALBUMID"))
	req.MusicBrainzReleaseGroupID = firstNonEmpty(req.MusicBrainzReleaseGroupID, comments.First("MUSICBRAINZ_RELEASEGROUPID"))
	req.MusicBrainzArtistIDs = appendTagValues(req.MusicBrainzArtistIDs, comments.Get("MUSICBRAINZ_ARTISTID")...)
	req.MusicBrainzAlbumArtistIDs = appendTagValues(req.MusicBrainzAlbumArtistIDs, comments.Get("MUSICBRAINZ_ALBUMARTISTID")...)
}

func applyMP4Freeform(req *domain.CreateMusicRequest, tag *mp4.Ilst) {
	if tag == nil {
		return
	}
	req.ReleaseDate = firstNonEmpty(req.ReleaseDate, tag.Date())
	for _, item := range tag.Items {
		if item == nil || item.Key != "----" || !strings.EqualFold(strings.TrimSpace(item.MeanDomain), "com.apple.iTunes") {
			continue
		}
		for _, data := range item.Data {
			if data != nil {
				applyNamedMetadataValue(req, item.Name, data.String())
			}
		}
	}
}

func applyAPETags(req *domain.CreateMusicRequest, items []ape.Item) {
	for _, item := range items {
		if item.Type == ape.ItemBinary {
			continue
		}
		for _, value := range item.Values() {
			applyNamedMetadataValue(req, item.Key, value)
		}
	}
}

func applyNamedMetadataValue(req *domain.CreateMusicRequest, name, value string) {
	key := strings.ToLower(strings.NewReplacer(" ", "", "_", "", "-", "", "/", "").Replace(strings.TrimSpace(name)))
	switch key {
	case "artist":
		req.Artists = appendTagValues(req.Artists, value)
	case "artists":
		req.Artists = appendTagValues(req.Artists, splitTagList(value)...)
	case "albumartist":
		req.AlbumArtists = appendTagValues(req.AlbumArtists, value)
	case "albumartists":
		req.AlbumArtists = appendTagValues(req.AlbumArtists, splitTagList(value)...)
	case "genre":
		req.Genres = appendTagValues(req.Genres, value)
	case "originaldate", "originalreleasedate", "wmoriginalreleasetime", "wmoriginalreleaseyear":
		req.OriginalReleaseDate = firstNonEmpty(req.OriginalReleaseDate, value)
	case "releasedate", "date", "wmreleasedate":
		req.ReleaseDate = firstNonEmpty(req.ReleaseDate, value)
	case "isrc", "wmisrc":
		req.ISRCs = appendTagValues(req.ISRCs, splitTagList(value)...)
	case "musicbrainztrackid", "musicbrainzrecordingid":
		req.MusicBrainzRecordingID = firstNonEmpty(req.MusicBrainzRecordingID, value)
	case "musicbrainzreleasetrackid":
		req.MusicBrainzTrackID = firstNonEmpty(req.MusicBrainzTrackID, value)
	case "musicbrainzalbumid", "musicbrainzreleaseid":
		req.MusicBrainzReleaseID = firstNonEmpty(req.MusicBrainzReleaseID, value)
	case "musicbrainzreleasegroupid":
		req.MusicBrainzReleaseGroupID = firstNonEmpty(req.MusicBrainzReleaseGroupID, value)
	case "musicbrainzartistid":
		req.MusicBrainzArtistIDs = appendTagValues(req.MusicBrainzArtistIDs, splitTagList(value)...)
	case "musicbrainzalbumartistid":
		req.MusicBrainzAlbumArtistIDs = appendTagValues(req.MusicBrainzAlbumArtistIDs, splitTagList(value)...)
	}
}

func readASFMetadata(file *os.File, maxTagBytes, maxCoverBytes int64) (*domain.CreateMusicRequest, *scannedCover, error) {
	req := &domain.CreateMusicRequest{}
	info, err := file.Stat()
	if err != nil {
		return req, nil, err
	}
	var header [24]byte
	if _, err := file.ReadAt(header[:], 0); err != nil {
		return req, nil, err
	}
	rawHeaderSize := binary.LittleEndian.Uint64(header[16:24])
	if rawHeaderSize > math.MaxInt64 {
		return req, nil, fmt.Errorf("ASF metadata header has unsafe size %d", rawHeaderSize)
	}
	headerSize := int64(rawHeaderSize)
	if headerSize < 30 || headerSize > info.Size() || headerSize > maxTagBytes {
		return req, nil, fmt.Errorf("ASF metadata header has unsafe size %d", headerSize)
	}
	var childCountBytes [4]byte
	if _, err := file.ReadAt(childCountBytes[:], 24); err != nil {
		return req, nil, err
	}
	childCount := binary.LittleEndian.Uint32(childCountBytes[:])
	if childCount > maxVorbisCommentCount || int64(childCount)*24 > headerSize-30 {
		return req, nil, fmt.Errorf("ASF metadata object count %d exceeds safe bounds", childCount)
	}
	parsed, err := asf.Read(io.NewSectionReader(file, 0, headerSize))
	if err != nil {
		return req, nil, err
	}
	req.Title = parsed.Title
	req.Artist = parsed.Artist()
	req.Album = parsed.Album()
	req.AlbumArtist = parsed.AlbumArtist()
	req.Year = parsed.Year()
	req.TrackNumber, req.TrackTotal = parsed.TrackNumber()
	req.DiscNumber, req.DiscTotal = parsed.DiscNumber()
	req.Genres = appendTagValues(req.Genres, parsed.Genre())
	req.Comment = parsed.Comment()
	for _, descriptor := range parsed.Extended {
		if descriptor.Type == asf.TypeString {
			applyNamedMetadataValue(req, descriptor.Name, descriptor.String())
		}
	}
	var cover *scannedCover
	for _, picture := range parsed.Pictures() {
		if len(picture.Data) == 0 || int64(len(picture.Data)) > maxCoverBytes {
			continue
		}
		candidate := &scannedCover{MIME: picture.MIME, Data: picture.Data, Type: tunetag.PictureType(picture.Type)}
		cover = preferScannedCover(cover, candidate)
		if candidate.Type == tunetag.PictureCoverFront {
			break
		}
	}
	sanitizeScannedMetadata(req)
	return req, cover, nil
}

func sanitizeScannedMetadata(req *domain.CreateMusicRequest) {
	req.Title = safeMetadataText(req.Title, 255)
	req.Artist = safeMetadataText(req.Artist, 255)
	req.Album = safeMetadataText(req.Album, 255)
	req.AlbumArtist = safeMetadataText(req.AlbumArtist, 255)
	req.Genre = safeMetadataText(req.Genre, 500)
	req.Comment = safeMetadataText(req.Comment, 16*1024)
	req.Artists = boundedMetadataValues(normalizeDisplayValues(req.Artists, false), 255)
	req.AlbumArtists = boundedMetadataValues(normalizeDisplayValues(req.AlbumArtists, false), 255)
	req.Genres = boundedMetadataValues(normalizeDisplayValues(req.Genres, false), 500)
	if req.Artist == "" && len(req.Artists) > 0 {
		req.Artist = joinMetadataDisplayValues(req.Artists, 255)
	}
	if req.AlbumArtist == "" && len(req.AlbumArtists) > 0 {
		req.AlbumArtist = joinMetadataDisplayValues(req.AlbumArtists, 255)
	}
	if len(req.Genres) == 0 && strings.TrimSpace(req.Genre) != "" {
		req.Genres = appendTagValues(req.Genres, req.Genre)
	}
	if req.Year < 1000 || req.Year > 9999 {
		req.Year = 0
	}
	req.ReleaseDate = safePartialDate(req.ReleaseDate)
	req.OriginalReleaseDate = safePartialDate(req.OriginalReleaseDate)
	if req.Year == 0 && len(req.ReleaseDate) >= 4 {
		req.Year, _ = strconv.Atoi(req.ReleaseDate[:4])
	}
	if req.TrackNumber < 0 {
		req.TrackNumber = 0
	}
	if req.TrackTotal < req.TrackNumber {
		req.TrackTotal = 0
	}
	if req.DiscNumber < 0 {
		req.DiscNumber = 0
	}
	if req.DiscTotal < req.DiscNumber {
		req.DiscTotal = 0
	}
	if req.Duration < 0 {
		req.Duration = 0
	}
	req.ISRCs = validISRCValues(req.ISRCs)
	req.MusicBrainzRecordingID = validMBIDValue(req.MusicBrainzRecordingID)
	req.MusicBrainzTrackID = validMBIDValue(req.MusicBrainzTrackID)
	req.MusicBrainzReleaseID = validMBIDValue(req.MusicBrainzReleaseID)
	req.MusicBrainzReleaseGroupID = validMBIDValue(req.MusicBrainzReleaseGroupID)
	req.MusicBrainzArtistIDs = validMBIDValues(req.MusicBrainzArtistIDs)
	req.MusicBrainzAlbumArtistIDs = validMBIDValues(req.MusicBrainzAlbumArtistIDs)
}

func boundedMetadataValues(values domain.StringList, maxBytes int) domain.StringList {
	if len(values) > maxCanonicalMetadataValues {
		values = values[:maxCanonicalMetadataValues]
	}
	result := make(domain.StringList, 0, len(values))
	for _, value := range values {
		if value = safeMetadataText(value, maxBytes); value != "" {
			result = append(result, value)
		}
	}
	return result
}

func safeMetadataText(value string, maxBytes int) string {
	value = strings.TrimSpace(strings.ToValidUTF8(value, "�"))
	value = strings.Map(func(char rune) rune {
		if unicode.IsControl(char) && char != '\n' && char != '\t' {
			return -1
		}
		return char
	}, value)
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value)
}

func safePartialDate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 10 && value[4] == '-' && value[7] == '-' {
		value = value[:10]
	} else if len(value) >= 7 && value[4] == '-' {
		value = value[:7]
	} else if len(value) >= 4 {
		value = value[:4]
	}
	if validatePartialDate("date", value) != nil {
		return ""
	}
	return value
}

func validISRCValues(values domain.StringList) domain.StringList {
	var valid domain.StringList
	for _, value := range values {
		for _, candidate := range splitTagList(value) {
			normalized, err := normalizeISRCs(domain.StringList{candidate})
			if err == nil {
				valid = appendTagValues(valid, normalized...)
			}
		}
	}
	return valid
}

func validMBIDValue(value string) string {
	normalized, err := normalizeMBID("musicbrainz_id", strings.TrimSpace(value))
	if err != nil {
		return ""
	}
	return normalized
}

func validMBIDValues(values domain.StringList) domain.StringList {
	var valid domain.StringList
	for _, value := range values {
		for _, candidate := range splitTagList(value) {
			if normalized := validMBIDValue(candidate); normalized != "" {
				valid = appendTagValues(valid, normalized)
			}
		}
	}
	return valid
}

func appendTagValues(current domain.StringList, values ...string) domain.StringList {
	for _, value := range values {
		value = strings.TrimSpace(strings.Trim(value, "\x00"))
		if value != "" {
			current = append(current, value)
		}
	}
	return current
}

func splitTagList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == 0 || r == ';' || r == ',' || r == '/'
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func flacDurationSeconds(file *flac.File) int {
	if file == nil {
		return 0
	}
	for _, block := range file.Blocks {
		raw, ok := block.(*flac.RawBlock)
		if !ok || raw.BlockType != flac.BlockStreamInfo || len(raw.Body) < 18 {
			continue
		}
		sampleRate := uint64(raw.Body[10])<<12 | uint64(raw.Body[11])<<4 | uint64(raw.Body[12])>>4
		totalSamples := uint64(raw.Body[13]&0x0f)<<32 |
			uint64(raw.Body[14])<<24 | uint64(raw.Body[15])<<16 | uint64(raw.Body[16])<<8 | uint64(raw.Body[17])
		if sampleRate > 0 {
			return durationSecondsFromUint64(roundedUint64Quotient(totalSamples, sampleRate))
		}
	}
	return 0
}

func roundedUint64Quotient(value, divisor uint64) uint64 {
	if divisor == 0 {
		return 0
	}
	quotient, remainder := value/divisor, value%divisor
	if remainder >= divisor/2+divisor%2 && quotient < math.MaxUint64 {
		quotient++
	}
	return quotient
}

func roundedInt64Quotient(value, divisor int64) int64 {
	if value <= 0 || divisor <= 0 {
		return 0
	}
	quotient, remainder := value/divisor, value%divisor
	if remainder >= divisor/2+divisor%2 && quotient < math.MaxInt64 {
		quotient++
	}
	return quotient
}

func durationSecondsFromUint64(value uint64) int {
	if value > uint64(math.MaxInt) {
		return math.MaxInt
	}
	return int(value)
}

func durationSecondsFromInt64(value int64) int {
	if value <= 0 {
		return 0
	}
	if value > int64(math.MaxInt) {
		return math.MaxInt
	}
	return int(value)
}

func readWAVMetadata(file *os.File, maxTagBytes int64) (*domain.CreateMusicRequest, *scannedCover, error) {
	req := &domain.CreateMusicRequest{}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return req, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return req, nil, err
	}
	var header [12]byte
	if _, err := io.ReadFull(file, header[:]); err != nil || string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return req, nil, errors.New("invalid RIFF/WAVE header")
	}

	var byteRate uint32
	var dataBytes int64
	var cover *scannedCover
	var metadataErr error
	for {
		var chunk [8]byte
		_, err := io.ReadFull(file, chunk[:])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			metadataErr = err
			break
		}
		size := int64(binary.LittleEndian.Uint32(chunk[4:8]))
		position, _ := file.Seek(0, io.SeekCurrent)
		if position < 0 || position > info.Size() || size > info.Size()-position {
			metadataErr = errors.New("WAV chunk exceeds file size")
			break
		}
		switch string(chunk[0:4]) {
		case "fmt ":
			body, readErr := readBoundedChunk(file, size, 64)
			if readErr != nil {
				metadataErr = readErr
			} else if len(body) >= 12 {
				byteRate = binary.LittleEndian.Uint32(body[8:12])
			}
		case "data":
			if size > math.MaxInt64-dataBytes {
				metadataErr = errors.New("WAV data size exceeds safe bounds")
				break
			}
			dataBytes += size
			_, err = file.Seek(size, io.SeekCurrent)
		case "LIST":
			body, readErr := readBoundedChunk(file, size, maxTextChunkBytes)
			if readErr != nil {
				metadataErr = readErr
			} else {
				applyRIFFInfo(req, body)
			}
		case "id3 ", "ID3 ":
			body, readErr := readBoundedChunk(file, size, maxTagBytes)
			if readErr != nil {
				metadataErr = readErr
			} else if tag, parseErr := id3v2.Read(bytes.NewReader(body)); parseErr != nil {
				metadataErr = parseErr
			} else {
				cover = applyID3Tag(req, tag)
			}
		default:
			_, err = file.Seek(size, io.SeekCurrent)
		}
		if err != nil {
			metadataErr = err
			break
		}
		if size%2 == 1 {
			if _, err := file.Seek(1, io.SeekCurrent); err != nil {
				metadataErr = err
				break
			}
		}
	}
	if byteRate > 0 {
		req.Duration = durationSecondsFromInt64(roundedInt64Quotient(dataBytes, int64(byteRate)))
	}
	sanitizeScannedMetadata(req)
	return req, cover, metadataErr
}

func applyRIFFInfo(req *domain.CreateMusicRequest, body []byte) {
	if len(body) < 4 || string(body[:4]) != "INFO" {
		return
	}
	for position := 4; position+8 <= len(body); {
		key := string(body[position : position+4])
		size := int(binary.LittleEndian.Uint32(body[position+4 : position+8]))
		position += 8
		if size < 0 || position+size > len(body) {
			return
		}
		value := strings.TrimSpace(strings.TrimRight(string(body[position:position+size]), "\x00"))
		switch key {
		case "INAM":
			req.Title = firstNonEmpty(req.Title, value)
		case "IART":
			req.Artist = firstNonEmpty(req.Artist, value)
		case "IPRD":
			req.Album = firstNonEmpty(req.Album, value)
		case "ICRD":
			req.ReleaseDate = firstNonEmpty(req.ReleaseDate, value)
		case "IGNR":
			req.Genres = appendTagValues(req.Genres, value)
		case "ICMT":
			req.Comment = firstNonEmpty(req.Comment, value)
		case "ITRK":
			req.TrackNumber, req.TrackTotal = parseTrackPair(value)
		}
		position += size
		if size%2 == 1 {
			position++
		}
	}
}

func readAIFFMetadata(file *os.File, maxTagBytes int64) (*domain.CreateMusicRequest, *scannedCover, error) {
	req := &domain.CreateMusicRequest{}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return req, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return req, nil, err
	}
	var header [12]byte
	if _, err := io.ReadFull(file, header[:]); err != nil || string(header[0:4]) != "FORM" || (string(header[8:12]) != "AIFF" && string(header[8:12]) != "AIFC") {
		return req, nil, errors.New("invalid AIFF header")
	}

	var cover *scannedCover
	var metadataErr error
	for {
		var chunk [8]byte
		_, err := io.ReadFull(file, chunk[:])
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			metadataErr = err
			break
		}
		size := int64(binary.BigEndian.Uint32(chunk[4:8]))
		position, _ := file.Seek(0, io.SeekCurrent)
		if position < 0 || position > info.Size() || size > info.Size()-position {
			metadataErr = errors.New("AIFF chunk exceeds file size")
			break
		}
		switch string(chunk[0:4]) {
		case "COMM":
			body, readErr := readBoundedChunk(file, size, 64)
			if readErr != nil {
				metadataErr = readErr
			} else if len(body) >= 18 {
				frames := uint64(binary.BigEndian.Uint32(body[2:6]))
				rate := extended80Float(body[8:18])
				if rate > 0 {
					req.Duration = int(math.Round(float64(frames) / rate))
				}
			}
		case "NAME", "AUTH", "ANNO":
			body, readErr := readBoundedChunk(file, size, maxTextChunkBytes)
			if readErr != nil {
				metadataErr = readErr
			} else {
				value := strings.TrimSpace(strings.TrimRight(string(body), "\x00"))
				switch string(chunk[0:4]) {
				case "NAME":
					req.Title = firstNonEmpty(req.Title, value)
				case "AUTH":
					req.Artist = firstNonEmpty(req.Artist, value)
				case "ANNO":
					req.Comment = firstNonEmpty(req.Comment, value)
				}
			}
		case "ID3 ":
			body, readErr := readBoundedChunk(file, size, maxTagBytes)
			if readErr != nil {
				metadataErr = readErr
			} else if tag, parseErr := id3v2.Read(bytes.NewReader(body)); parseErr != nil {
				metadataErr = parseErr
			} else {
				cover = applyID3Tag(req, tag)
			}
		default:
			_, err = file.Seek(size, io.SeekCurrent)
		}
		if err != nil {
			metadataErr = err
			break
		}
		if size%2 == 1 {
			if _, err := file.Seek(1, io.SeekCurrent); err != nil {
				metadataErr = err
				break
			}
		}
	}
	sanitizeScannedMetadata(req)
	return req, cover, metadataErr
}

func readBoundedChunk(reader io.ReadSeeker, size, limit int64) ([]byte, error) {
	if size < 0 {
		return nil, errors.New("metadata chunk has a negative size")
	}
	if size > limit {
		_, err := reader.Seek(size, io.SeekCurrent)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("metadata chunk exceeds %d bytes", limit)
	}
	body := make([]byte, size)
	if _, err := io.ReadFull(reader, body); err != nil {
		return nil, err
	}
	return body, nil
}

func extended80Float(value []byte) float64 {
	if len(value) < 10 {
		return 0
	}
	exponent := int(binary.BigEndian.Uint16(value[:2]) & 0x7fff)
	mantissa := binary.BigEndian.Uint64(value[2:10])
	if exponent == 0 || mantissa == 0 {
		return 0
	}
	sign := 1.0
	if value[0]&0x80 != 0 {
		sign = -1
	}
	return sign * math.Ldexp(float64(mantissa), exponent-16383-63)
}

func parseTrackPair(value string) (int, int) {
	parts := strings.SplitN(strings.TrimSpace(value), "/", 2)
	number, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
	total := 0
	if len(parts) == 2 {
		total, _ = strconv.Atoi(strings.TrimSpace(parts[1]))
	}
	return number, total
}

func mp4DurationSeconds(path string) int {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return 0
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return 0
	}
	moovStart, moovSize, ok := findMP4Box(file, 0, info.Size(), "moov")
	if !ok {
		return 0
	}
	mvhdStart, mvhdSize, ok := findMP4Box(file, moovStart, moovSize, "mvhd")
	if !ok || mvhdSize < 20 {
		return 0
	}
	buffer := make([]byte, 32)
	n, err := file.ReadAt(buffer, mvhdStart)
	if err != nil && !errors.Is(err, io.EOF) {
		return 0
	}
	buffer = buffer[:n]
	if len(buffer) < 20 {
		return 0
	}
	if buffer[0] == 1 {
		if len(buffer) < 32 {
			return 0
		}
		timeScale := uint64(binary.BigEndian.Uint32(buffer[20:24]))
		duration := binary.BigEndian.Uint64(buffer[24:32])
		if timeScale > 0 {
			return durationSecondsFromUint64(roundedUint64Quotient(duration, timeScale))
		}
		return 0
	}
	timeScale := uint64(binary.BigEndian.Uint32(buffer[12:16]))
	duration := uint64(binary.BigEndian.Uint32(buffer[16:20]))
	if timeScale > 0 {
		return durationSecondsFromUint64(roundedUint64Quotient(duration, timeScale))
	}
	return 0
}

func findMP4Box(reader io.ReaderAt, start, length int64, wanted string) (int64, int64, bool) {
	position := start
	end := start + length
	for position+8 <= end {
		var header [16]byte
		if _, err := reader.ReadAt(header[:8], position); err != nil {
			return 0, 0, false
		}
		size := int64(binary.BigEndian.Uint32(header[:4]))
		headerSize := int64(8)
		switch size {
		case 1:
			if _, err := reader.ReadAt(header[8:16], position+8); err != nil {
				return 0, 0, false
			}
			extendedSize := binary.BigEndian.Uint64(header[8:16])
			if extendedSize > math.MaxInt64 {
				return 0, 0, false
			}
			size = int64(extendedSize)
			headerSize = 16
		case 0:
			size = end - position
		}
		if size < headerSize || position+size > end {
			return 0, 0, false
		}
		if string(header[4:8]) == wanted {
			return position + headerSize, size - headerSize, true
		}
		position += size
	}
	return 0, 0, false
}

func oggDurationSeconds(file *os.File) int {
	info, err := file.Stat()
	if err != nil || info.Size() < 27 {
		return 0
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return 0
	}
	head := make([]byte, minInt64(info.Size(), 64*1024))
	if _, err := io.ReadFull(file, head); err != nil {
		return 0
	}
	sampleRate := uint64(0)
	preSkip := uint64(0)
	if index := bytes.Index(head, []byte{1, 'v', 'o', 'r', 'b', 'i', 's'}); index >= 0 && index+16 <= len(head) {
		sampleRate = uint64(binary.LittleEndian.Uint32(head[index+12 : index+16]))
	} else if index := bytes.Index(head, []byte("OpusHead")); index >= 0 && index+12 <= len(head) {
		sampleRate = 48000
		preSkip = uint64(binary.LittleEndian.Uint16(head[index+10 : index+12]))
	}
	if sampleRate == 0 {
		return 0
	}
	tailSize := minInt64(info.Size(), 128*1024)
	tail := make([]byte, tailSize)
	if _, err := file.ReadAt(tail, info.Size()-tailSize); err != nil && !errors.Is(err, io.EOF) {
		return 0
	}
	for index := len(tail) - 27; index >= 0; index-- {
		if string(tail[index:index+4]) != "OggS" || tail[index+4] != 0 {
			continue
		}
		segmentCount := int(tail[index+26])
		if index+27+segmentCount > len(tail) {
			continue
		}
		bodySize := 0
		for _, size := range tail[index+27 : index+27+segmentCount] {
			bodySize += int(size)
		}
		if index+27+segmentCount+bodySize > len(tail) {
			continue
		}
		granule := binary.LittleEndian.Uint64(tail[index+6 : index+14])
		if granule > preSkip {
			return durationSecondsFromUint64(roundedUint64Quotient(granule-preSkip, sampleRate))
		}
	}
	return 0
}

func minInt64(left, right int64) int64 {
	if left < right {
		return left
	}
	return right
}

func saveScannedCover(musicID uint, picture *scannedCover, uploadDir string, maxCoverBytes int64) (string, error) {
	if picture == nil || len(picture.Data) == 0 {
		return "", nil
	}
	if maxCoverBytes <= 0 || int64(len(picture.Data)) > maxCoverBytes {
		return "", fmt.Errorf("embedded cover exceeds configured limit")
	}
	detected := http.DetectContentType(picture.Data[:minInt(len(picture.Data), signatureReadSize)])
	if _, ok := allowedCoverMIMEs[detected]; !ok {
		return "", fmt.Errorf("embedded cover has unsupported content type %q", detected)
	}
	extension := map[string]string{
		"image/gif": ".gif", "image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp",
	}[detected]
	if strings.TrimSpace(uploadDir) == "" {
		return "", errors.New("managed upload directory is not configured")
	}
	directory := filepath.Join(uploadDir, strconv.FormatUint(uint64(musicID), 10))
	if err := os.MkdirAll(directory, 0750); err != nil {
		return "", err
	}
	target := filepath.Join(directory, "cover"+extension)
	temporary, err := os.CreateTemp(directory, ".cover-*")
	if err != nil {
		return "", err
	}
	temporaryPath := temporary.Name()
	keep := false
	defer func() {
		_ = temporary.Close()
		if !keep {
			_ = os.Remove(temporaryPath)
		}
	}()
	if err := temporary.Chmod(0600); err != nil {
		return "", err
	}
	if _, err := temporary.Write(picture.Data); err != nil {
		return "", err
	}
	if err := temporary.Sync(); err != nil {
		return "", err
	}
	if err := temporary.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return "", err
	}
	keep = true
	return target, nil
}

func minInt(left, right int) int {
	if left < right {
		return left
	}
	return right
}
