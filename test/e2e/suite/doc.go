// Package suite blank-imports every end-to-end suite so that each one's init()
// reaches the registry of the framework.
//
// A suite that is not imported here does not exist as far as TestE2E is
// concerned, so adding a suite means adding a line.
package suite
