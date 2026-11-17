package main

import (
	"fmt"
	"sort"
	"strings"
)

// errorRules returns the compared rules raised as errors, sorted.
func errorRules(r result) []string {
	var out []string
	for _, rule := range sortedRules(r.Rules) {
		if r.Rules[rule] == levelError {
			out = append(out, rule)
		}
	}
	return out
}

// sameError reports whether both sides raise at least one common error.
func sameError(c comparison) bool {
	for _, r := range errorsOf(c.Ref.Rules) {
		for _, m := range errorsOf(c.Ours.Rules) {
			if same(r, m) {
				return true
			}
		}
	}
	return false
}

// level gives, for a difference, the level at which the other side raises
// the rule (or its equivalent) when it raises it as a warning or an
// information: "+CII-SR-465" → "`+CII-SR-465` (warning)".
func level(c comparison, d string) string {
	rule := d[1:]
	var other string
	switch d[0] {
	case '+':
		for r, l := range c.Ref.Rules {
			if same(r, rule) {
				other = l
			}
		}
	case '-':
		for m, l := range c.Ours.Rules {
			if same(rule, m) {
				other = l
			}
		}
	default:
		return "`" + d + "`"
	}
	if other == "" {
		return "`" + d + "`"
	}
	return fmt.Sprintf("`%s` (%s)", d, other)
}

func cell(rules []string) string {
	if len(rules) == 0 {
		return "—"
	}
	return "`" + strings.Join(rules, "` `") + "`"
}

func markdown(all []comparison, unused []string) string {
	var b strings.Builder
	b.WriteString("| Référence | Factures comparées | Mêmes erreurs | Écarts justifiés | Écarts inattendus | Même verdict |\n|---|---|---|---|---|---|\n")
	for _, ref := range references {
		var n, same, known, unexpected, verdict int
		for _, c := range all {
			if c.Reference != ref.Key {
				continue
			}
			n++
			switch {
			case len(c.Unexpected) > 0:
				unexpected++
			case len(c.Known) > 0:
				known++
			default:
				same++
			}
			if c.Ours.valid() == c.Ref.valid() {
				verdict++
			}
		}
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d | %d |\n", ref.Title, n, same, known, unexpected, verdict)
	}
	b.WriteString("\nPour chaque facture, les règles levées en erreur de part et d'autre sont comparées : celles des schematrons officiels (EN 16931, Factur-X, XRechnung, Peppol), sous leur identifiant ou, pour les listes de codes des schematrons Factur-X, sous celui de la règle CEN équivalente, et la présence d'erreurs de schéma XSD (`SCHEMA`). Le verdict est valide quand aucune de ces règles n'est en erreur.\n")

	// Negative cases: one line per invoice, one column per reference.
	type neg struct {
		refs map[string]comparison
		ours result
	}
	negs := map[string]*neg{}
	var names []string
	for _, c := range all {
		if !c.Negative {
			continue
		}
		n := negs[c.Invoice]
		if n == nil {
			n = &neg{refs: map[string]comparison{}, ours: c.Ours}
			negs[c.Invoice] = n
			names = append(names, c.Invoice)
		}
		n.refs[c.Reference] = c
	}
	sort.Strings(names)
	if len(names) > 0 {
		b.WriteString("\n### Cas négatifs officiels\n\nFactures invalides à dessein : dossiers `fail` du corpus ZUGFeRD et exemple `not_validating` de `XML-Rechnung/UBL`. Erreurs levées, parmi les règles comparées.\n\n| Facture | factur-x |")
		for _, ref := range references {
			fmt.Fprintf(&b, " %s |", ref.Key)
		}
		b.WriteString(" Même erreur |\n|---|---|")
		for range references {
			b.WriteString("---|")
		}
		b.WriteString("---|\n")
		for _, name := range names {
			n := negs[name]
			fmt.Fprintf(&b, "| `%s` | %s |", strings.TrimPrefix(name, "corpus/"), cell(errorRules(n.ours)))
			verdict := "—"
			for _, ref := range references {
				c, ok := n.refs[ref.Key]
				if !ok {
					b.WriteString(" hors périmètre |")
					continue
				}
				fmt.Fprintf(&b, " %s |", cell(errorRules(c.Ref)))
				if len(errorRules(c.Ref)) > 0 {
					if sameError(c) {
						if verdict == "—" {
							verdict = "oui"
						}
					} else {
						verdict = "non"
					}
				}
			}
			fmt.Fprintf(&b, " %s |\n", verdict)
		}
	}

	var knownLines, unexpectedLines []string
	for _, c := range all {
		for _, k := range c.Known {
			d, why, _ := strings.Cut(k, " : ")
			knownLines = append(knownLines, fmt.Sprintf("| %s | `%s` | %s | %s |", c.Reference, strings.TrimPrefix(c.Invoice, "corpus/"), level(c, d), why))
		}
		if len(c.Unexpected) > 0 {
			var ds []string
			for _, d := range c.Unexpected {
				ds = append(ds, level(c, d))
			}
			unexpectedLines = append(unexpectedLines, fmt.Sprintf("| %s | `%s` | %s |", c.Reference, strings.TrimPrefix(c.Invoice, "corpus/"), strings.Join(ds, " ")))
		}
	}
	const legend = "`+` : erreur levée par factur-x seul ; `-` : erreur levée par la référence seule ; entre parenthèses, le niveau auquel l'autre côté lève la règle quand ce n'est pas une erreur.\n\n"
	if len(knownLines) > 0 {
		b.WriteString("\n### Écarts justifiés\n\n" + legend + "| Référence | Facture | Écart | Raison |\n|---|---|---|---|\n")
		b.WriteString(strings.Join(knownLines, "\n") + "\n")
	}
	if len(unexpectedLines) > 0 {
		b.WriteString("\n### Écarts inattendus\n\n" + legend + "| Référence | Facture | Écarts |\n|---|---|---|\n")
		b.WriteString(strings.Join(unexpectedLines, "\n") + "\n")
	}
	if len(unused) > 0 {
		b.WriteString("\n### Écarts connus devenus sans objet\n\n")
		for _, u := range unused {
			b.WriteString("- " + u + "\n")
		}
	}
	return b.String()
}
