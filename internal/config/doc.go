// Package config loads user settings and per-instance device keys.
//
// Settings live in a JSON file the user edits. The app never writes that
// file, so signing in cannot silently change which server you talk to.
//
// Resolution order for every setting is: command-line flag, environment
// variable, config file, then the built-in default. The default server is
// production (https://dev-null.sh). Point a test or local instance with
// --url, DEVNULL_URL, or the "url" field.
//
// Device keys are stored separately under instances/<host>/ so a key
// minted for production cannot be reused against (or stolen by) another
// host. Request signatures do not include the host, which is why this
// split matters.
package config
