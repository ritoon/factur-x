// Package en16931 implements the business rules of EN 16931-1 for the CII syntax, as published by CEN TC 434 (validation artefacts 1.3.x): BR-*, BR-CO-*, BR-DEC-*, VAT category rules and code lists.
package en16931

import "github.com/ritoon/factur-x/internal/rules"

// Catalog lists the rules implemented by this package.
var Catalog []rules.RuleInfo

// Check runs the rules on c.Doc and records the findings in c.Report.
func Check(c *rules.Checker) {}
