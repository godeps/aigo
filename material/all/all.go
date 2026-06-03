// Package all imports all search backends for their registration side effects.
// Import this package to make all backends available via search.NewFromURIs.
//
//	import _ "github.com/godeps/aigo/material/all"
package all

import (
	_ "github.com/godeps/aigo/material/local"
	_ "github.com/godeps/aigo/material/ossmeta"
	_ "github.com/godeps/aigo/material/pexels"
	_ "github.com/godeps/aigo/material/pixabay"
	_ "github.com/godeps/aigo/material/unsplash"
)
