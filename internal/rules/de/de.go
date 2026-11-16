// Package de implements the German rules BR-DE of XRechnung for the CII syntax.
package de

import "github.com/ritoon/factur-x/internal/rules"

// Catalog lists the rules implemented by this package.
var Catalog []rules.RuleInfo

// Check runs the rules on c.Doc and records the findings in c.Report.
func Check(c *rules.Checker) {}
