# factur-x

Package Go pour les factures électroniques hybrides **Factur-X**, **ZUGFeRD** et **XRechnung**. Il prend un PDF déjà produit par votre application et y ajoute ce que la norme exige ; il relit et valide n'importe quelle facture de ces formats, quel que soit son profil.

- Pur Go, bibliothèque standard uniquement : pas de CGO, pas de JVM, pas de binaire externe, pas d'appel réseau.
- Factur-X 1.0x = ZUGFeRD 2.x, syntaxe CII, profils MINIMUM, BASIC WL, BASIC, EN 16931, EXTENDED, XRECHNUNG et EXTENDED-CTC-FR.
- Validation complète sans Java : schéma du profil, règles EN 16931, règles de profil Factur-X, règles françaises BR-FR (réforme 2026) et allemandes BR-DE (XRechnung), contrôle du conteneur PDF.
- Licence MIT.

> **Statut : première version (v0).** L'API peut encore changer. Voir [Couverture](#couverture) pour ce qui est fait et [Limites](#limites) pour ce qui reste.

Le contexte, l'état de l'écosystème Go et les mesures sont sur le wiki : [Pourquoi ce projet](https://github.com/ritoon/factur-x/wiki/Pourquoi-ce-projet), [Grille de la norme](https://github.com/ritoon/factur-x/wiki/Grille-de-la-norme), [Benchmarks](https://github.com/ritoon/factur-x/wiki/Benchmarks).

## Installation

```bash
go get github.com/ritoon/factur-x
```

Go 1.24 ou plus récent.

## Utilisation

### Créer une facture Factur-X

Votre application produit le PDF visuel (idéalement en PDF/A-3) ; le package écrit le XML CII du profil voulu et l'injecte.

```go
d := invoice.MustDecimal
inv := &invoice.Invoice{
	Profile:   invoice.ProfileEN16931,
	Number:    "FA-2026-00042",
	TypeCode:  invoice.TypeCommercialInvoice,
	IssueDate: invoice.NewDate(2026, time.October, 15),
	Currency:  "EUR",
	Seller:    invoice.Party{Name: "Exemple Informatique SAS", VATID: "FR32123456789" /* … */},
	Buyer:     invoice.Party{Name: "Client Exemple SARL" /* … */},
	Lines: []invoice.Line{{
		ID:              "1",
		Item:            invoice.Item{Name: "Maintenance applicative, octobre 2026"},
		Quantity:        d("2"),
		UnitCode:        "DAY",
		NetPrice:        d("450.00"),
		VATCategoryCode: invoice.VATStandard,
		VATRate:         d("20.00"),
	}},
}

pdf, err := facturx.Create(host, inv, &facturx.CreateOptions{
	Calculate: true, // montants de ligne, totaux et ventilation de TVA
	Validate:  &validate.Options{Rules: validate.Default | validate.France},
})
```

- `Calculate` remplit les montants que la norme déduit des autres (BT-131, BT-106 à BT-115, ventilation BG-23), en décimaux exacts.
- Avec `Validate`, `Create` refuse d'injecter un XML qui a des erreurs et renvoie un `*facturx.ValidationError` qui porte le rapport complet.
- L'injection se fait par mise à jour incrémentale : les octets d'origine sont conservés, une signature existante reste valide. Le XMP est fusionné avec l'existant (PDF/A-3, propriétés `fx`, schéma d'extension).

L'exemple complet, avec les mentions obligatoires de la réforme française, est dans [example_test.go](example_test.go).

Pour injecter un XML produit ailleurs : `facturx.Embed(pdf, xml, nil)`. Pour écrire le XML seul : `cii.Marshal(inv, &cii.MarshalOptions{Profile: invoice.ProfileBasic})` ; seuls les éléments que le profil autorise sont écrits, dans l'ordre de son schéma, et l'option `Strict` signale les données perdues.

### Lire une facture reçue

```go
inv, att, err := facturx.Read(pdf)
// inv : modèle EN 16931 (inv.Number, inv.Seller, inv.Lines, inv.Totals…)
// att : nom du fichier, AFRelationship, présence dans /AF, métadonnées XMP
```

`facturx.Extract` renvoie seulement le XML. `cii.Parse` lit un XML CII seul. La lecture est tolérante : ZUGFeRD 2.0 et 2.1, préfixes quelconques, identifiants de profil mal orthographiés trouvés dans la nature ; une valeur illisible est signalée sans arrêter la lecture du reste.

### Valider

```go
report, att, err := facturx.Check(pdf, &validate.Options{Auto: true})
for _, f := range report.Findings {
	fmt.Println(f.Severity, f.Rule, f.Message, f.Path, f.Line)
}
if !report.Valid() { /* au moins une erreur */ }
```

`Check` contrôle le conteneur PDF puis le XML. Pour un XML seul : `validate.XML(xml, opts)`. Le rapport ne s'arrête jamais à la première erreur ; chaque constat porte l'identifiant officiel de la règle, sa gravité, sa source, le chemin XPath et la ligne.

| Option | Effet |
|---|---|
| `nil` ou `&validate.Options{}` | Schéma du profil, règles EN 16931, règles de profil Factur-X ; règles BR-DE en plus pour une facture XRECHNUNG |
| `Auto: true` | Ajoute les règles BR-FR quand le vendeur est en France (BT-40 = FR) |
| `Rules: validate.Default \| validate.France` | Choix explicite des jeux de règles |
| `Profile: invoice.ProfileBasic` | Valide contre un profil plutôt que celui que déclare BT-24 |

`validate.Catalog(set)` liste les règles implémentées.

### En ligne de commande

```bash
go install github.com/ritoon/factur-x/cmd/facturx@latest
```

```text
facturx extract [-o out.xml] facture.pdf
facturx info facture.pdf
facturx validate [-fr] [-de] [-auto] [-profile p] [-json] facture.pdf|facture.xml
facturx embed -xml facture.xml [-profile p] [-o out.pdf] entree.pdf
facturx json facture.pdf|facture.xml
facturx rules [-fr] [-de]
```

`validate` sort avec le code 1 quand la facture a des erreurs, ce qui permet de l'utiliser en intégration continue.

```text
$ facturx validate testdata/EN16931_Einfach.pdf
Profile: EN 16931
warning [PDF] PDF-06: the catalog has no PDF/A output intent (/S /GTS_PDFA1) (catalog /OutputIntents)
0 error(s), 1 warning(s)
```

## Couverture

### Profils

| Profil | Lecture | Écriture | Schéma | Règles de profil |
|---|---|---|---|---|
| MINIMUM | oui | oui | oui | oui |
| BASIC WL | oui | oui | oui | oui |
| BASIC | oui | oui | oui | oui |
| EN 16931 | oui | oui | oui | oui |
| EXTENDED | oui | oui | oui | oui |
| XRECHNUNG (CII) | oui | oui | schéma EN 16931 | oui ; BR-DE appliquées d'office |
| EXTENDED-CTC-FR | oui | oui | schéma EXTENDED | oui ; BR-FR sur option (`France` ou `Auto`) |

### Règles

| Jeu | Règles | Référence |
|---|---|---|
| Schéma | 11 contrôles (éléments, ordre, cardinalités, types, listes fermées) | XSD Factur-X 1.08, compilés en tables Go |
| EN 16931 | 793 assertions (328 erreurs, 465 avertissements) : BR, BR-CO, BR-CL, BR-DEC, règles par catégorie de TVA (BR-S, BR-Z, BR-E, BR-AE, BR-IC, BR-IG, BR-IP, BR-G, BR-O, BR-AF, BR-AG, BR-B), CII-SR, CII-DT | Artefacts CEN TC 434 v1.3.17 (CII) |
| Factur-X | 162 assertions : identification du profil (BT-24), assertions des schematrons MINIMUM et BASIC WL, éléments non utilisés par profil | Schematrons Factur-X 1.08 |
| France (BR-FR) | 59 règles, 170 assertions : SIREN/SIRET, adresses électroniques, mentions obligatoires, cadres de facturation, multi-vendeurs | Schematron FNFE-MPE v1.4.0.04 |
| Allemagne (BR-DE) | 52 assertions (BR-DE, BR-DE-CVD, BR-DEX, BR-TMP) | Schematron KoSIT XRechnung 3.0.2 |
| Conteneur PDF | 28 contrôles (PDF-01 à PDF-28) : PDF/A-3 déclaré, XMP `fx` cohérent avec le XML, pièce jointe, `/AF`, `/AFRelationship`, type MIME, chiffrement | Spécification Factur-X 1.08, ISO 19005-3 |

Les messages sont ceux des artefacts officiels. Les listes de codes (pays, devises, unités, schémas d'identifiants, motifs d'exonération…) sont embarquées.

### Exactitude mesurée

Sur les 26 XML CII valides du corpus officiel `XML-Rechnung/CII`, le validateur signale exactement les 6 fichiers que signalent les schematrons officiels (exécutés par papierleicht) : BR-CL-25 deux fois, CII-SR-465 quatre fois sur les exemples XRECHNUNG. Les 104 PDF hybrides du corpus (`ZUGFeRDv2/correct`, `XML-Rechnung/FX`) sont tous extraits et leurs 26 XML tous lus.

### PDF

Le lecteur PDF est écrit pour ce package : tables xref classiques et flux xref, flux d'objets, mises à jour incrémentales, reconstruction quand la table xref est endommagée, filtres Flate (avec prédicteurs), ASCIIHex et ASCII85. La décompression est bornée (256 Mio par flux) et la profondeur d'imbrication limitée, pour qu'un fichier hostile ne puisse ni saturer la mémoire ni faire déborder la pile. Le décompresseur est testé par fuzzing différentiel contre `compress/flate`.

Le lecteur XML est lui aussi écrit pour ce package : un seul passage sur les octets, espaces de noms résolus, imbrication limitée à 256 niveaux, déclarations DOCTYPE ignorées et aucune entité développée hors des cinq prédéfinies. Il est testé par fuzzing différentiel contre `encoding/xml` : mêmes documents acceptés, même arbre.

## Limites

- **Pas de conversion PDF/A.** `Embed` déclare PDF/A-3 dans le XMP mais ne touche ni aux polices, ni aux espaces de couleur, ni aux output intents : le PDF hôte doit déjà respecter PDF/A-2 ou PDF/A-3 (polices incorporées, output intent), ce que la plupart des générateurs savent faire sur option. `Check` signale ce qui manque.
- **Pas d'UBL.** XRechnung et la réforme française acceptent aussi l'UBL ; seule la syntaxe CII est traitée.
- **PDF chiffrés** : signalés (PDF-01), pas déchiffrés.
- **ZUGFeRD 1.0** : le XMP est reconnu, le XML (syntaxe CII antérieure à D16B) n'est pas lu.
- Les règles des schematrons sont réécrites en Go, pas interprétées : une nouvelle version des artefacts officiels demande une mise à jour du code (le catalogue EN 16931 est régénéré par `go generate`, les tests vérifient l'alignement).

## Performances

Apple M4 Pro, Go 1.26, 16/11/2026. Comparaison avec le plus rapide des autres packages Go qui réussit l'opération ; tableaux complets dans [bench/ecosystem/results/2026-11-16/tables.md](bench/ecosystem/results/2026-11-16/tables.md).

| Opération | factur-x | Meilleur autre package Go |
|---|---|---|
| Extraction du XML (PDF de 149 Ko) | 28 µs | 75 µs (speedata) |
| Lecture XML → modèle (13 Ko) | 15 µs | 126 µs (fX) |
| Lecture et validation complète (13 Ko) | 23 µs | 132 µs (pdf0 + formalis, sans règles françaises) |
| Lecture et validation complète (1,3 Mo, pièces jointes base64) | 744 µs | 7,01 ms (angelo) |
| Écriture modèle → XML (13 Ko) | 2,7 µs | 20,7 µs (fX) |
| Injection dans un PDF à flux xref | 80 µs | 543 µs (pdfcpu) |
| Contrôle complet d'un PDF hybride (149 Ko) | 55 µs | 1,18 ms (DCNWS) |

Le harnais est dans [bench/ecosystem](bench/ecosystem) ; chaque librairie y a son propre module.

## Organisation

| Package | Rôle |
|---|---|
| `facturx` (racine) | Création, lecture, extraction, injection et contrôle d'un PDF hybride |
| `invoice` | Modèle EN 16931 (BT, BG), décimaux exacts, dates, profils, calcul des totaux |
| `cii` | Lecture et écriture du XML CII, profil par profil |
| `validate` | Validation : schéma, règles EN 16931, règles de profil Factur-X, règles BR-FR et BR-DE |
| `cmd/facturx` | Outil en ligne de commande |
| `internal/…` | Lecteur et écrivain PDF, XMP, arbre XML, règles, listes de codes, tables de schéma |
| `bench/ecosystem` | Benchmarks comparatifs avec les autres librairies Go (modules séparés) |

## Tests

```bash
go test ./...
```

Les tests de base n'ont besoin que du dépôt ; la CI ([.github/workflows/ci.yml](.github/workflows/ci.yml)) les exécute avec `-race` sur Go 1.24 et sur la dernière version, après `gofmt` et `go vet`, et fait tourner 30 secondes de fuzzing sur le lecteur PDF, le codec Flate et le lecteur XML. Des tests supplémentaires s'activent quand les artefacts officiels sont disponibles en local :

| Variable | Contenu | Tests |
|---|---|---|
| `FX_CORPUS` | Clone de [ZUGFeRD/corpus](https://github.com/ZUGFeRD/corpus) | Extraction et contrôle des PDF du corpus |
| `FX_CORPUS_XML` | XML extraits du corpus | Lecture, validation et aller-retour lecture → écriture |
| `FX_XSD` | XSD du paquet Factur-X 1.08, un dossier par profil | Le XML écrit par `cii.Marshal` est vérifié contre les XSD d'origine |
| `FX_XSD_DIR` | Paquet Factur-X 1.08 | Les tables de schéma régénérées sont identiques à celles du dépôt |
| `FX_KOSIT` | Clone de [itplr-kosit/xrechnung-testsuite](https://github.com/itplr-kosit/xrechnung-testsuite) | Toutes les factures CII de la suite de tests KoSIT passent les règles BR-DE |

## Licence et provenance

Le code est sous licence MIT (voir [LICENSE](LICENSE)).

Les règles sont réécrites en Go à partir des artefacts officiels, qui ne sont pas inclus dans le dépôt ; leurs identifiants et leurs messages sont repris pour que les rapports correspondent aux validateurs de référence :

| Artefact | Version | Licence |
|---|---|---|
| Artefacts de validation EN 16931 (CEN TC 434, ConnectingEurope/eInvoicing-EN16931) | 1.3.17 | EUPL-1.2 |
| Schematron BR-FR (FNFE-MPE, fnfempe/France_RFE) | 1.4.0.04 | EUPL-1.2 |
| Schematron XRechnung (KoSIT) | 3.0.2 | Apache-2.0 |
| Schémas et schematrons Factur-X (FNFE-MPE / FeRD) | 1.08 | Apache-2.0 |

Les fichiers de `testdata/` à la racine viennent du corpus ZUGFeRD (Apache-2.0), voir [testdata/NOTICE](testdata/NOTICE). Les factures de test des règles françaises et allemandes ont été écrites à la main, avec des parties fictives.
