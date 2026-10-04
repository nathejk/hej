package video

import (
	"errors"
	"testing"
)

const iphoneProbe = `{"streams":[
 {"codec_type":"video","width":3840,"height":2160,"side_data_list":[{"rotation":-90}]},
 {"codec_type":"audio"},
 {"codec_type":"data"}],
 "format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"1320.042000"}}`

func TestParseProbeReadsAnIphoneClip(t *testing.T) {
	info, err := parseProbe([]byte(iphoneProbe))
	if err != nil {
		t.Fatal(err)
	}
	if info.DurationMs != 1_320_042 || info.Width != 3840 || info.Height != 2160 || info.Rotation != 270 || !info.HasAudio {
		t.Errorf("got %+v", info)
	}
	if got := ContentType(info, "IMG_0001.MOV"); got != "video/quicktime" {
		t.Errorf("content type %s", got)
	}
}

// An mp3 with cover art has a "video" stream. It is not a video.
func TestParseProbeRefusesCoverArt(t *testing.T) {
	_, err := parseProbe([]byte(`{"streams":[{"codec_type":"audio"},
		{"codec_type":"video","width":600,"height":600,"disposition":{"attached_pic":1}}],
		"format":{"format_name":"mp3","duration":"180"}}`))
	if !errors.Is(err, ErrNotVideo) {
		t.Errorf("want ErrNotVideo, got %v", err)
	}
}

func TestParseProbeReadsTheLegacyRotateTag(t *testing.T) {
	info, err := parseProbe([]byte(`{"streams":[{"codec_type":"video","width":1920,"height":1080,"tags":{"rotate":"90"}}],
		"format":{"format_name":"mov,mp4,m4a,3gp,3g2,mj2","duration":"12.5"}}`))
	if err != nil || info.Rotation != 90 || info.DurationMs != 12_500 {
		t.Errorf("got %+v, %v", info, err)
	}
}
