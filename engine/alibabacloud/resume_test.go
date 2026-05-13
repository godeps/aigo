package alibabacloud

import (
	"reflect"
	"testing"
)

// TestExtractorForModel_AllTableEntriesCovered ensures every model registered
// in modelTable has an explicit case in extractorForModel — preventing the
// "default fallthrough returns wrong URL field" bug class.
func TestExtractorForModel_AllTableEntriesCovered(t *testing.T) {
	want := map[string][][]string{
		// image
		ModelQwenImage:         {{"results", "url"}, {"result_url"}},
		ModelQwenImage2:        {{"results", "url"}, {"result_url"}},
		ModelQwenImageEditPlus: {{"results", "url"}, {"result_url"}},
		ModelWanImage:          {{"results", "url"}, {"result_url"}},
		ModelZImageTurbo:       {{"results", "url"}, {"result_url"}},
		// asr
		ModelQwenASRFlash:          {{"results", "transcription_url"}, {"results", "text"}},
		ModelQwenASRFlashFiletrans: {{"results", "transcription_url"}, {"results", "text"}},
		// 3d
		ModelTripoP1:  {{"results", "pbr_model_url"}},
		ModelTripoH31: {{"results", "pbr_model_url"}},
		// video (wan + kling)
		ModelWanTextToVideo:    {{"video_url"}},
		ModelWanImageToVideo:   {{"video_url"}},
		ModelWanReferenceVideo: {{"video_url"}},
		ModelWanVideoEdit:      {{"video_url"}},
		ModelKlingV3Video:      {{"video_url"}},
		ModelKlingV3OmniVideo:  {{"video_url"}},
		// video (happyhorse)
		ModelHappyHorseT2V:       {{"video_url"}},
		ModelHappyHorseI2V:       {{"video_url"}},
		ModelHappyHorseR2V:       {{"video_url"}},
		ModelHappyHorseVideoEdit: {{"video_url"}},
		// sync-only handlers: empty extractor (never reach Resume)
		ModelQwenTTSFlash:         nil,
		ModelQwenTTSInstructFlash: nil,
		ModelQwenVoiceDesign:      nil,
		ModelFunMusic:             nil,
	}

	for model := range modelTable {
		expected, known := want[model]
		if !known {
			t.Errorf("modelTable contains %q but resume_test.go has no expectation — add it to TestExtractorForModel_AllTableEntriesCovered", model)
			continue
		}
		got := extractorForModel(model).URLFields
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("extractorForModel(%q) URLFields = %v, want %v", model, got, expected)
		}
	}

	for model := range want {
		if _, ok := modelTable[model]; !ok {
			t.Errorf("test expects model %q but it is missing from modelTable", model)
		}
	}
}

// TestExtractorForModel_UnknownFallback documents the conservative video_url
// fallback for genuinely unknown models. New models must be added explicitly
// to extractorForModel rather than relying on this default.
func TestExtractorForModel_UnknownFallback(t *testing.T) {
	got := extractorForModel("not-a-real-model").URLFields
	want := [][]string{{"video_url"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractorForModel(unknown) = %v, want %v", got, want)
	}
}
