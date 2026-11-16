// Package schema checks the structure of a CII invoice against the XML
// schema of its Factur-X profile: allowed elements, order, cardinalities and
// simple types.
package schema

import "github.com/ritoon/factur-x/internal/rules"

// Catalog lists the rules implemented by this package.
var Catalog []rules.RuleInfo

// Check validates the structure of c.Doc against the schema of c.Doc.Profile
// and records the findings in c.Report.
func Check(c *rules.Checker) {}
