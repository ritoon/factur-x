// Package facturx implements the Factur-X / ZUGFeRD profile rules: what each profile requires and allows beyond EN 16931.
package facturx

import "github.com/ritoon/factur-x/internal/rules"

// Catalog lists the rules implemented by this package.
var Catalog []rules.RuleInfo

// Check runs the rules on c.Doc and records the findings in c.Report.
func Check(c *rules.Checker) {}
