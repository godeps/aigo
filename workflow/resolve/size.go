package resolve

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/godeps/aigo/workflow"
)

// Dimensions represents pixel width and height.
type Dimensions struct {
	Width  int
	Height int
}

// SizeSpec is the canonical representation of size-related information
// extracted from a workflow graph. Engines use it to convert between formats.
type SizeSpec struct {
	Dimensions  *Dimensions
	AspectRatio string // "W:H" (e.g. "16:9")
	Resolution  string // "720P", "1080P", etc.
}

// IsZero returns true if the spec contains no usable size information.
func (s SizeSpec) IsZero() bool {
	return s.Dimensions == nil && s.AspectRatio == "" && s.Resolution == ""
}

var (
	reWxH        = regexp.MustCompile(`^(\d+)\s*[xX×]\s*(\d+)$`)
	reWAsteriskH = regexp.MustCompile(`^(\d+)\s*\*\s*(\d+)$`)
	reRatio      = regexp.MustCompile(`^(\d+)\s*:\s*(\d+)$`)
	reResolution = regexp.MustCompile(`^(\d+)[pP]$`)
)

// ParseSize parses any known size format into a SizeSpec.
// Supported formats: "1024x1024", "1024*1024", "16:9", "720P", "720p".
func ParseSize(raw string) SizeSpec {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return SizeSpec{}
	}

	// Try WxH (letter x/X/×)
	if m := reWxH.FindStringSubmatch(raw); m != nil {
		w, _ := strconv.Atoi(m[1])
		h, _ := strconv.Atoi(m[2])
		if w > 0 && h > 0 {
			return ParseDimensions(w, h)
		}
	}

	// Try W*H (asterisk)
	if m := reWAsteriskH.FindStringSubmatch(raw); m != nil {
		w, _ := strconv.Atoi(m[1])
		h, _ := strconv.Atoi(m[2])
		if w > 0 && h > 0 {
			return ParseDimensions(w, h)
		}
	}

	// Try aspect ratio "W:H"
	if m := reRatio.FindStringSubmatch(raw); m != nil {
		rw, _ := strconv.Atoi(m[1])
		rh, _ := strconv.Atoi(m[2])
		if rw > 0 && rh > 0 {
			return SizeSpec{AspectRatio: fmt.Sprintf("%d:%d", rw, rh)}
		}
	}

	// Try resolution "720P" / "1080P"
	if m := reResolution.FindStringSubmatch(raw); m != nil {
		h, _ := strconv.Atoi(m[1])
		if h > 0 {
			return SizeSpec{Resolution: fmt.Sprintf("%dP", h)}
		}
	}

	return SizeSpec{}
}

// ParseDimensions creates a SizeSpec from pixel dimensions,
// automatically deriving the simplified aspect ratio.
func ParseDimensions(w, h int) SizeSpec {
	if w <= 0 || h <= 0 {
		return SizeSpec{}
	}
	return SizeSpec{
		Dimensions:  &Dimensions{Width: w, Height: h},
		AspectRatio: simplifyRatio(w, h),
		Resolution:  deriveResolution(w, h),
	}
}

// ToWxH returns the "WxH" format (e.g. "1024x1024"). Empty if no dimensions.
func (s SizeSpec) ToWxH() string {
	if s.Dimensions == nil {
		return ""
	}
	return fmt.Sprintf("%dx%d", s.Dimensions.Width, s.Dimensions.Height)
}

// ToWAsteriskH returns the "W*H" format (e.g. "1024*1024"). Empty if no dimensions.
func (s SizeSpec) ToWAsteriskH() string {
	if s.Dimensions == nil {
		return ""
	}
	return fmt.Sprintf("%d*%d", s.Dimensions.Width, s.Dimensions.Height)
}

// ToAspectRatio returns the simplified aspect ratio (e.g. "16:9").
func (s SizeSpec) ToAspectRatio() string {
	if s.AspectRatio != "" {
		return s.AspectRatio
	}
	if s.Dimensions != nil {
		return simplifyRatio(s.Dimensions.Width, s.Dimensions.Height)
	}
	return ""
}

// ToResolution returns the resolution label (e.g. "720P", "1080P").
func (s SizeSpec) ToResolution() string {
	if s.Resolution != "" {
		return s.Resolution
	}
	if s.Dimensions != nil {
		return deriveResolution(s.Dimensions.Width, s.Dimensions.Height)
	}
	return ""
}

// ToWidthHeight returns the pixel dimensions. Returns (0, 0) if unavailable.
func (s SizeSpec) ToWidthHeight() (int, int) {
	if s.Dimensions != nil {
		return s.Dimensions.Width, s.Dimensions.Height
	}
	return 0, 0
}

// SnapTo finds the nearest supported size from the given list.
// Distance metric: aspect-ratio angle difference (primary), pixel area difference (secondary).
// Returns the original spec if supported is empty or no valid entry is found.
func (s SizeSpec) SnapTo(supported []string) SizeSpec {
	if len(supported) == 0 || s.IsZero() {
		return s
	}

	candidates := make([]SizeSpec, 0, len(supported))
	for _, entry := range supported {
		c := ParseSize(entry)
		if !c.IsZero() {
			candidates = append(candidates, c)
		}
	}
	if len(candidates) == 0 {
		return s
	}

	inputAngle := s.aspectAngle()
	inputPixels := s.pixelArea()

	bestIdx := 0
	bestAngleDiff := math.MaxFloat64
	bestPixelDiff := math.MaxFloat64

	const angleEpsilon = 0.01 // ~0.6 degrees

	for i, c := range candidates {
		angleDiff := math.Abs(c.aspectAngle() - inputAngle)
		pixelDiff := math.Abs(float64(c.pixelArea() - inputPixels))

		if angleDiff < bestAngleDiff-angleEpsilon ||
			(math.Abs(angleDiff-bestAngleDiff) < angleEpsilon && pixelDiff < bestPixelDiff) {
			bestIdx = i
			bestAngleDiff = angleDiff
			bestPixelDiff = pixelDiff
		}
	}

	return candidates[bestIdx]
}

// NormalizeImageSize snaps width/height to the nearest supported image size string ("WxH").
// This is the replacement for the legacy NormalizeOpenAIImageSize when a supported list is available.
func NormalizeImageSize(supported []string, w, h int) string {
	spec := ParseDimensions(w, h)
	if spec.IsZero() {
		if len(supported) > 0 {
			return supported[0]
		}
		return "1024x1024"
	}
	result := spec.SnapTo(supported)
	if s := result.ToWxH(); s != "" {
		return s
	}
	return "1024x1024"
}

// NormalizeVideoSize snaps width/height to the nearest supported video dimensions.
func NormalizeVideoSize(supported []string, w, h int) (int, int) {
	spec := ParseDimensions(w, h)
	if spec.IsZero() {
		if len(supported) > 0 {
			first := ParseSize(supported[0])
			if first.Dimensions != nil {
				return first.Dimensions.Width, first.Dimensions.Height
			}
		}
		return w, h
	}
	result := spec.SnapTo(supported)
	if result.Dimensions != nil {
		return result.Dimensions.Width, result.Dimensions.Height
	}
	return w, h
}

// ExtractImageSizeSpec extracts a unified SizeSpec for image tasks from the graph.
// Priority: ImageOptions node > global "size" option > EmptyLatentImage dimensions.
func ExtractImageSizeSpec(g workflow.Graph) SizeSpec {
	// 1. ImageOptions node
	for _, ref := range g.FindByClassType("ImageOptions") {
		if s, ok := ref.Node.StringInput("size"); ok && s != "" {
			spec := ParseSize(s)
			if !spec.IsZero() {
				// Also check for aspect_ratio in the same node
				if ar, ok := ref.Node.StringInput("aspect_ratio"); ok && ar != "" {
					spec.AspectRatio = ar
				}
				if res, ok := ref.Node.StringInput("resolution"); ok && res != "" {
					spec.Resolution = res
				}
				return spec
			}
		}
		// No size but has aspect_ratio
		if ar, ok := ref.Node.StringInput("aspect_ratio"); ok && ar != "" {
			spec := SizeSpec{AspectRatio: ar}
			if res, ok := ref.Node.StringInput("resolution"); ok && res != "" {
				spec.Resolution = res
			}
			return spec
		}
	}

	// 2. Global "size" option
	if s, ok := StringOption(g, "size"); ok && s != "" {
		spec := ParseSize(s)
		if !spec.IsZero() {
			return spec
		}
	}

	// 3. Global "aspect_ratio" option
	if ar, ok := StringOption(g, "aspect_ratio"); ok && ar != "" {
		return SizeSpec{AspectRatio: ar}
	}

	// 4. Global width/height integers
	if w, okW := IntOption(g, "width"); okW && w > 0 {
		if h, okH := IntOption(g, "height"); okH && h > 0 {
			return ParseDimensions(w, h)
		}
	}

	// 5. EmptyLatentImage node
	for _, ref := range g.FindByClassType("EmptyLatentImage") {
		w, okW := ref.Node.IntInput("width")
		h, okH := ref.Node.IntInput("height")
		if okW && okH && w > 0 && h > 0 {
			return ParseDimensions(w, h)
		}
	}

	return SizeSpec{}
}

// ExtractVideoSizeSpec extracts a unified SizeSpec for video tasks from the graph.
// Priority: VideoOptions node > global options > EmptyLatentImage dimensions.
func ExtractVideoSizeSpec(g workflow.Graph) SizeSpec {
	// 1. VideoOptions node
	for _, ref := range g.FindByClassType("VideoOptions") {
		var spec SizeSpec

		// Try explicit width/height first
		w, okW := ref.Node.IntInput("width")
		h, okH := ref.Node.IntInput("height")
		if okW && okH && w > 0 && h > 0 {
			spec = ParseDimensions(w, h)
		}

		// Try "size" string
		if spec.Dimensions == nil {
			if s, ok := ref.Node.StringInput("size"); ok && s != "" {
				parsed := ParseSize(s)
				if parsed.Dimensions != nil {
					spec.Dimensions = parsed.Dimensions
				}
				if parsed.AspectRatio != "" && spec.AspectRatio == "" {
					spec.AspectRatio = parsed.AspectRatio
				}
				if parsed.Resolution != "" && spec.Resolution == "" {
					spec.Resolution = parsed.Resolution
				}
			}
		}

		// Overlay aspect_ratio
		if ar, ok := ref.Node.StringInput("aspect_ratio"); ok && ar != "" {
			spec.AspectRatio = ar
		}

		// Overlay resolution
		if res, ok := ref.Node.StringInput("resolution"); ok && res != "" {
			spec.Resolution = res
		}

		if !spec.IsZero() {
			return spec
		}
	}

	// 2. Global options
	if s, ok := StringOption(g, "size"); ok && s != "" {
		spec := ParseSize(s)
		if ar, ok := StringOption(g, "aspect_ratio"); ok && ar != "" {
			spec.AspectRatio = ar
		}
		if res, ok := StringOption(g, "resolution"); ok && res != "" {
			spec.Resolution = res
		}
		if !spec.IsZero() {
			return spec
		}
	}

	// 2b. Global width/height integers
	if w, okW := IntOption(g, "width"); okW && w > 0 {
		if h, okH := IntOption(g, "height"); okH && h > 0 {
			spec := ParseDimensions(w, h)
			if ar, ok := StringOption(g, "aspect_ratio"); ok && ar != "" {
				spec.AspectRatio = ar
			}
			if res, ok := StringOption(g, "resolution"); ok && res != "" {
				spec.Resolution = res
			}
			return spec
		}
	}

	if ar, ok := StringOption(g, "aspect_ratio"); ok && ar != "" {
		spec := SizeSpec{AspectRatio: ar}
		if res, ok := StringOption(g, "resolution"); ok && res != "" {
			spec.Resolution = res
		}
		return spec
	}

	if res, ok := StringOption(g, "resolution"); ok && res != "" {
		return SizeSpec{Resolution: res}
	}

	// 3. EmptyLatentImage
	for _, ref := range g.FindByClassType("EmptyLatentImage") {
		w, okW := ref.Node.IntInput("width")
		h, okH := ref.Node.IntInput("height")
		if okW && okH && w > 0 && h > 0 {
			return ParseDimensions(w, h)
		}
	}

	return SizeSpec{}
}

// --- internal helpers ---

func gcd(a, b int) int {
	for b != 0 {
		a, b = b, a%b
	}
	return a
}

func simplifyRatio(w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	d := gcd(w, h)
	rw, rh := w/d, h/d
	// Limit to common ratios to avoid ugly numbers like "128:96"
	if rw <= 32 && rh <= 32 {
		return fmt.Sprintf("%d:%d", rw, rh)
	}
	// Fall back to approximate well-known ratios
	ratio := float64(w) / float64(h)
	switch {
	case math.Abs(ratio-16.0/9.0) < 0.02:
		return "16:9"
	case math.Abs(ratio-9.0/16.0) < 0.02:
		return "9:16"
	case math.Abs(ratio-4.0/3.0) < 0.02:
		return "4:3"
	case math.Abs(ratio-3.0/4.0) < 0.02:
		return "3:4"
	case math.Abs(ratio-3.0/2.0) < 0.02:
		return "3:2"
	case math.Abs(ratio-2.0/3.0) < 0.02:
		return "2:3"
	case math.Abs(ratio-1.0) < 0.02:
		return "1:1"
	default:
		return fmt.Sprintf("%d:%d", rw, rh)
	}
}

func deriveResolution(w, h int) string {
	short := w
	if h < w {
		short = h
	}
	switch {
	case short >= 2160:
		return "4K"
	case short >= 1440:
		return "2K"
	case short >= 1080:
		return "1080P"
	case short >= 720:
		return "720P"
	case short >= 480:
		return "480P"
	default:
		return ""
	}
}

// aspectAngle returns the aspect ratio as an angle for distance comparison.
// Uses atan2(h, w) so landscape < π/4 < portrait.
func (s SizeSpec) aspectAngle() float64 {
	if s.Dimensions != nil && s.Dimensions.Width > 0 && s.Dimensions.Height > 0 {
		return math.Atan2(float64(s.Dimensions.Height), float64(s.Dimensions.Width))
	}
	if s.AspectRatio != "" {
		if m := reRatio.FindStringSubmatch(s.AspectRatio); m != nil {
			rw, _ := strconv.Atoi(m[1])
			rh, _ := strconv.Atoi(m[2])
			if rw > 0 && rh > 0 {
				return math.Atan2(float64(rh), float64(rw))
			}
		}
	}
	return math.Pi / 4 // default: square
}

// pixelArea returns total pixel count for proximity comparison.
func (s SizeSpec) pixelArea() int {
	if s.Dimensions != nil {
		return s.Dimensions.Width * s.Dimensions.Height
	}
	// For aspect-ratio-only specs, use a reference resolution for comparison
	if s.Resolution != "" {
		if m := reResolution.FindStringSubmatch(s.Resolution); m != nil {
			h, _ := strconv.Atoi(m[1])
			// Assume 16:9 for pixel estimation
			w := h * 16 / 9
			return w * h
		}
	}
	return 0
}
