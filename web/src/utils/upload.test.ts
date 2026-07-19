import type { ICommonTagsResult, IFormat } from "music-metadata";
import { describe, expect, it } from "vitest";
import { extractMetaFields, metadataToData } from "./upload";

describe("Picard-compatible upload metadata", () => {
  it("keeps multi-value tags and distinct MusicBrainz entity IDs", () => {
    const common: ICommonTagsResult = {
      album: "Release",
      albumartist: "Album Artist",
      albumartists: ["Album Artist", "Guest Curator"],
      artist: "Artist feat. Guest",
      artists: ["Artist", "Guest"],
      comment: [{ text: "liner note" }],
      date: "2024-03-02",
      disk: { no: 1, of: 2 },
      genre: ["Ambient / Chillout", "Electronic"],
      isrc: ["USABC2412345"],
      musicbrainz_albumartistid: ["123e4567-e89b-42d3-a456-426614174005"],
      musicbrainz_albumid: "123e4567-e89b-42d3-a456-426614174002",
      musicbrainz_artistid: ["123e4567-e89b-42d3-a456-426614174004"],
      musicbrainz_recordingid: "123e4567-e89b-42d3-a456-426614174000",
      musicbrainz_releasegroupid: "123e4567-e89b-42d3-a456-426614174003",
      musicbrainz_trackid: "123e4567-e89b-42d3-a456-426614174001",
      movementIndex: { no: null, of: null },
      originaldate: "2023",
      title: "Track",
      track: { no: 2, of: 12 },
      year: 2024,
    };

    const fields = extractMetaFields(common, { duration: 201.4 } as IFormat);
    const data = metadataToData(fields);

    expect(data).toMatchObject({
      album_artist: "Album Artist",
      album_artists: ["Album Artist", "Guest Curator"],
      artists: ["Artist", "Guest"],
      disc_number: 1,
      disc_total: 2,
      genres: ["Ambient / Chillout", "Electronic"],
      musicbrainz_recording_id: common.musicbrainz_recordingid,
      musicbrainz_release_id: common.musicbrainz_albumid,
      musicbrainz_track_id: common.musicbrainz_trackid,
      original_release_date: "2023",
      release_date: "2024-03-02",
      track_number: 2,
      track_total: 12,
    });
  });
});
