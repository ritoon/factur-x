// Package fr implements the French rules BR-FR (FNFE-MPE schematrons for the 2026 e-invoicing reform) for the CII syntax.
package fr

import "github.com/ritoon/factur-x/internal/rules"

// Catalog lists the rules implemented by this package.
var Catalog []rules.RuleInfo

// Check runs the rules on c.Doc and records the findings in c.Report.
func Check(c *rules.Checker) {}
