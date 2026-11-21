# ZeRechnung

[![Go Reference](https://pkg.go.dev/badge/github.com/ritoon/zerechnung.svg)](https://pkg.go.dev/github.com/ritoon/zerechnung)
[![CI](https://github.com/ritoon/zerechnung/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/ritoon/zerechnung/actions/workflows/ci.yml)
[![Conformité](https://github.com/ritoon/zerechnung/actions/workflows/conformance.yml/badge.svg?branch=main)](https://github.com/ritoon/zerechnung/actions/workflows/conformance.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/ritoon/zerechnung)](https://goreportcard.com/report/github.com/ritoon/zerechnung)
[![codecov](https://codecov.io/gh/ritoon/zerechnung/graph/badge.svg)](https://codecov.io/gh/ritoon/zerechnung)
[![Version de Go](https://img.shields.io/github/go-mod/go-version/ritoon/zerechnung)](go.mod)
[![Licence MIT](https://img.shields.io/github/license/ritoon/zerechnung)](LICENSE)

<p align="center">
  <img src="docs/logo.png" alt="ZeRechnung, Go Package for Factur-X &amp; ZUGFeRD : le gopher Go lit une facture à côté d'une machine à écrire, un bateau en papier coiffe le n, une pile de factures marquée FR et DE">
</p>

Package Go pour les factures électroniques hybrides **Factur-X**, **ZUGFeRD** et **XRechnung**. Il prend un PDF déjà produit par votre application et y ajoute ce que la norme exige ; il relit et valide n'importe quelle facture de ces formats, quel que soit son profil. Il lit, écrit et valide aussi les factures XML en syntaxe **UBL 2.1**.

- Pur Go, bibliothèque standard uniquement : pas de CGO, pas de JVM, pas de binaire externe, pas d'appel réseau.
- Factur-X 1.0x = ZUGFeRD 2.x, syntaxe CII, profils MINIMUM, BASIC WL, BASIC, EN 16931, EXTENDED, XRECHNUNG et EXTENDED-CTC-FR ; schémas et règles de profil à jour de Factur-X 1.09.2 (ZUGFeRD 2.5.2). Factures Peppol BIS Billing 3.0 et NLCIUS (Pays-Bas) en CII et en UBL.
- UBL 2.1 (Invoice et CreditNote, liaison EN 16931 de la CEN/TS 16931-3-2) : lecture et écriture dans le même modèle que le CII, validation contre le schéma XSD UBL 2.1 et selon les artefacts EN 16931 pour UBL ; la syntaxe d'un XML est détectée d'après son élément racine.
- Validation complète sans Java : schéma XSD (du profil Factur-X, ou UBL 2.1), règles EN 16931 (CII et UBL), règles de profil Factur-X, règles françaises BR-FR (réforme 2026), allemandes BR-DE (XRechnung) et néerlandaises BR-NL (NLCIUS, CII et UBL), règles Peppol BIS Billing 3.0 (CII et UBL), contrôle du conteneur PDF.
- Licence MIT.

> **Statut : première version (v0).** L'API peut encore changer. Voir [Couverture](#couverture) pour ce qui est fait et [Limites](#limites) pour ce qui reste.

Le contexte, l'état de l'écosystème Go et les mesures sont sur le wiki : [Pourquoi ce projet](https://github.com/ritoon/zerechnung/wiki/Pourquoi-ce-projet), [Grille de la norme](https://github.com/ritoon/zerechnung/wiki/Grille-de-la-norme), [Benchmarks](https://github.com/ritoon/zerechnung/wiki/Benchmarks).

## Installation

```bash
go get github.com/ritoon/zerechnung
```

Go 1.24 ou plus récent.

## Utilisation

### Créer une facture Factur-X

Votre application produit le PDF visuel (idéalement en PDF/A-3, ou en PDF/A-4 pour un PDF 2.0) ; le package écrit le XML CII du profil voulu et l'injecte.

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

pdf, err := zerechnung.Create(host, inv, &zerechnung.CreateOptions{
	Calculate: true, // montants de ligne, totaux et ventilation de TVA
	Validate:  &validate.Options{Rules: validate.Default | validate.France},
})
```

- `Calculate` remplit les montants que la norme déduit des autres (BT-131, BT-106 à BT-115, ventilation BG-23), en décimaux exacts.
- Avec `Validate`, `Create` refuse d'injecter un XML qui a des erreurs et renvoie un `*zerechnung.ValidationError` qui porte le rapport complet.
- L'injection se fait par mise à jour incrémentale : les octets d'origine sont conservés, une signature existante reste valide. Le XMP est fusionné avec l'existant (identification PDF/A, propriétés `fx`, schéma d'extension) : un PDF 2.0 ou un hôte déjà PDF/A-4 est déclaré PDF/A-4f (le niveau E est conservé), les autres PDF/A-3.

L'exemple complet, avec les mentions obligatoires de la réforme française, est dans [example_test.go](example_test.go).

#### TVA sur les débits

Un prestataire de services qui a opté pour le paiement de la TVA d'après les débits doit le mentionner sur ses factures (article 242 nonies A de l'annexe II au CGI). La DGFiP fait porter cette mention par BT-8, avec le code de la date de facture : 5 en CII, 3 en UBL (règle G1.43 des spécifications externes v3.2).

```go
inv.TaxPointDateCode = invoice.TaxPointDebits // "5", écrit dans chaque ventilation de TVA
```

- Sans option, BT-8 reste vide.
- BT-8 exclut la date d'exigibilité BT-7 (BR-CO-03).
- Aucune règle ne vérifie la mention : la facture ne dit pas si le fournisseur a opté, et la DGFiP indique que la règle ne peut pas être contrôlée par les applications. Les contrôles possibles sur le XML existent déjà : liste de codes (BR-CL-06), une seule valeur (CII-SR-462), exclusion de BT-7 (BR-CO-03).
- Les codes 29 et 72 restent disponibles (`TaxPointDeliveryDate`, `TaxPointPaymentDate`). Les spécifications v2.3 exprimaient l'option par 29 ; la DGFiP est passée à 5 avec la v2.4.

L'exemple `Example_vatOnDebits` de [example_test.go](example_test.go) écrit et valide une telle facture.

Pour injecter un XML produit ailleurs : `zerechnung.Embed(pdf, xml, nil)`. Pour écrire le XML seul : `cii.Marshal(inv, &cii.MarshalOptions{Profile: invoice.ProfileBasic})` ; seuls les éléments que le profil autorise sont écrits, dans l'ordre de son schéma, et l'option `Strict` signale les données perdues.

#### Pièces jointes complémentaires

Factur-X admet d'autres pièces jointes que la facture : bon de commande, relevé de temps, bon de livraison… `EmbedOptions.Attachments` (et donc `CreateOptions.Embed`) les écrit comme le XML : flux avec type MIME et date de modification, `/F` et `/UF`, `/AFRelationship /Supplement` par défaut, entrée dans `/EmbeddedFiles` et dans le `/AF` du catalogue.

```go
pdf, err := zerechnung.Embed(host, xml, &zerechnung.EmbedOptions{
	Attachments: []zerechnung.File{
		{Name: "commande-4512.pdf", Data: commande, Description: "Bon de commande 4512"},
		{Name: "releve-temps.csv", Data: releve}, // type MIME déduit de l'extension : text/csv
	},
})
```

- Sans `MimeType`, le type est déduit de l'extension (PDF, XML, CSV, texte, JSON, images, ZIP, OpenDocument, Office), `application/octet-stream` à défaut. Sans `ModDate`, la date est celle de la facture.
- Les noms de la facture (`factur-x.xml`, `xrechnung.xml`, `zugferd-invoice.xml`, ou `EmbedOptions.Filename`) sont refusés, quelle que soit la casse. Une pièce jointe de l'hôte qui porte le même nom qu'une pièce jointe ajoutée est remplacée.
- `Check` signale une pièce jointe sans `/AFRelationship` (PDF-36), sans type MIME (PDF-37) ou absente de tout tableau `/AF` (PDF-39).

### Lire une facture reçue

```go
inv, att, err := zerechnung.Read(pdf)
// inv : modèle EN 16931 (inv.Number, inv.Seller, inv.Lines, inv.Totals…)
// att : nom du fichier, AFRelationship, présence dans /AF, métadonnées XMP
```

`zerechnung.Extract` renvoie seulement le XML. `cii.Parse` lit un XML CII seul, `ubl.Parse` une facture ou un avoir UBL 2.1 (voir [UBL](#ubl)). La lecture est tolérante : ZUGFeRD 2.0 et 2.1, ZUGFeRD 1.0 (voir [ZUGFeRD 1.0](#zugferd-10)), préfixes quelconques, identifiants de profil mal orthographiés trouvés dans la nature ; une valeur illisible est signalée sans arrêter la lecture du reste.

#### Rien n'est perdu en silence

Le modèle porte les données métier d'EXTENDED : parties supplémentaires (utilisateur final, livraison finale, expédition), conditions de livraison, commandes du client final, bon de livraison, taux de change, frais logistiques, remises et charges détaillées sur le prix, produits inclus dans un article… Les champs EXT-FR-FE-* d'EXTENDED-CTC-FR correspondent à des éléments du XSD EXTENDED et sont couverts de la même façon.

Ce que le modèle n'a pas de champ pour décrire (un numéro de fax, une marque de produit, des conditions d'escompte, un `listID`, un élément qu'aucun schéma ne connaît…) est conservé tel quel dans le champ `Extensions` de la valeur qui l'englobe : la facture, une ligne, une partie, un moyen de paiement… `inv.Unmodeled()` les liste :

```go
inv, _ := cii.Parse(xml)
for _, u := range inv.Unmodeled() {
	fmt.Println(u) // "Seller: DefinedTradeContact/FaxUniversalCommunication", "Lines[0]: SpecifiedTradeProduct/BrandName"…
}
```

`cii.Marshal` remet chaque extension à sa place dans l'ordre du schéma, en recréant les éléments englobants s'il le faut, quand le profil visé l'admet ; sinon il la signale comme perdue (option `Strict`). Une facture EXTENDED relue puis réécrite en EXTENDED redonne donc tout son arbre, aux formes normalisées près : `currencyID` égal à BT-5 ou BT-6 sous-entendu, attributs d'autres espaces de noms (`xsi:schemaLocation`) non conservés, immatriculations fiscales d'une partie écrites VA puis FC, identifiant global d'une partie écrit en `ram:GlobalID`, BT-7 et BT-8 répétés dans chaque ventilation de TVA. La liste des éléments et attributs du XSD EXTENDED sans champ dans le modèle est tenue dans [cii/testdata/extended-unmodeled.txt](cii/testdata/extended-unmodeled.txt).

#### UBL

```go
inv, err := ubl.Parse(xml)        // Invoice ou CreditNote UBL 2.1 → même modèle que cii.Parse
out, err := ubl.Marshal(inv, nil) // CreditNote si BT-3 est un code d'avoir, Invoice sinon
// UBL → CII
xmlCII, err := cii.Marshal(inv, &cii.MarshalOptions{Profile: invoice.ProfileEN16931})
```

- La lecture suit la liaison UBL d'EN 16931 (CEN/TS 16931-3-2) et accepte n'importe quel préfixe d'espace de noms ; une valeur illisible est signalée (`ubl.Issue`) sans arrêter la lecture.
- `Marshal` écrit le jeu de données EN 16931 dans l'ordre du schéma UBL 2.1. Ce que la liaison ne porte pas (les champs EXTENDED du modèle, les extensions) n'est pas écrit ; l'option `Strict` le signale (`*ubl.LossError`), comme pour `cii.Marshal`.
- Une facture lue dans une syntaxe s'écrit dans l'autre : le modèle est le même.

#### ZUGFeRD 1.0

`cii.Parse` et `zerechnung.Read` lisent aussi le XML des PDF ZUGFeRD 1.0 : `rsm:CrossIndustryDocument`, la syntaxe CII de 2013, antérieure à D16B, et `rsm:Invoice` de la release candidate de 2014. Le document est lu à travers une vue CII D16B construite en mémoire, sans le modifier : éléments renommés et remis dans l'ordre du schéma, dates écrites au format 102 (ZUGFeRD 1.0 admet aussi 610 et 616), référence du mandat SEPA et identifiant du créancier, portés par le moyen de paiement, déplacés en BT-89 et BT-90. Une valeur illisible est située dans le document ZUGFeRD 1.0.

| Identifiant ZUGFeRD 1.0 | Profil du modèle |
|---|---|
| `urn:ferd:CrossIndustryDocument:invoice:1p0:basic` | BASIC |
| `urn:ferd:CrossIndustryDocument:invoice:1p0:comfort` | EN 16931 |
| `urn:ferd:CrossIndustryDocument:invoice:1p0:extended` | EXTENDED |

Les identifiants de la release candidate (`urn:ferd:invoice:rc:basic`, `comfort`, `extended`) sont rapprochés de la même façon, et `inv.SpecificationID` garde l'identifiant lu. ZUGFeRD 1.0 est en lecture seule : `cii.Marshal` écrit la facture en Factur-X, avec l'identifiant du profil visé. Ce que CII D16B ne porte pas (nom de l'établissement bancaire, code banque allemand, type de taxe autre que `VAT`…) reste dans les extensions et est signalé comme perdu (option `Strict`).

ZUGFeRD 1.0 précède EN 16931 et CII D16B : les règles EN 16931, Factur-X et nationales, écrites pour des factures qui les suivent, donneraient de faux constats sur une facture ZUGFeRD 1.0 valide. `validate` ne lui applique donc qu'un contrôle structurel minimal, dans le jeu `Schema` : un identifiant ZUGFeRD 1.0 connu (ZF1-01), des valeurs lisibles (ZF1-02) et les données qui identifient une facture : numéro, type, date, vendeur, acheteur, devise et montant total (ZF1-03). L'avertissement ZF1-04, émis quels que soient les jeux choisis, rappelle qu'une facture ZUGFeRD 1.0 n'est pas une facture EN 16931 : le BMF n'accepte ZUGFeRD comme e-facture qu'à partir de la version 2.0.1, et la réforme française exige un format conforme à EN 16931.

### Valider

```go
report, att, err := zerechnung.Check(pdf, &validate.Options{Auto: true})
for _, f := range report.Findings {
	fmt.Println(f.Severity, f.Rule, f.Message, f.Path, f.Line)
}
if !report.Valid() { /* au moins une erreur */ }
```

`Check` contrôle le conteneur PDF puis le XML. Pour un XML seul : `validate.XML(xml, opts)`. Le rapport ne s'arrête jamais à la première erreur ; chaque constat porte l'identifiant officiel de la règle, sa gravité, sa source, le chemin XPath et la ligne.

`validate.XML` détecte la syntaxe. Un `Invoice` ou un `CreditNote` UBL 2.1 est vérifié contre le schéma XSD OASIS UBL 2.1 de son document, avec les mêmes règles XSD-* que le CII, puis contrôlé selon les artefacts EN 16931 pour UBL : règles métier et listes de codes avec leurs contextes UBL, règles de syntaxe UBL-CR (éléments et attributs qu'EN 16931 n'utilise pas, en avertissement), UBL-SR (cardinalités) et UBL-DT (types de données). Les règles de profil Factur-X, propres au CII, ne s'appliquent pas. Les règles BR-FR, BR-DE, Peppol et BR-NL suivent les schematrons UBL de leurs jeux, avec leurs identifiants, leurs contextes et leurs messages, et sont vérifiées sur le document UBL lui-même (voir [Peppol BIS Billing 3.0](#peppol-bis-billing-30)) ; les règles Chorus Pro s'appliquent à une vue CII du document. Tous les constats sont situés dans l'UBL. Un PDF Factur-X qui embarque un UBL est refusé : Factur-X impose le CII.

| Option | Effet |
|---|---|
| `nil` ou `&validate.Options{}` | Schéma (du profil, CII D16B pour XRECHNUNG, ou UBL 2.1), règles EN 16931, règles de profil Factur-X ; règles BR-DE en plus pour une facture XRECHNUNG, règles Peppol pour une facture Peppol BIS Billing 3.0, règles BR-NL pour une facture NLCIUS (CII ou UBL) |
| `Auto: true` | Ajoute les règles du pays du vendeur (BT-40) : BR-FR et profils admis en France pour un vendeur en France, profils acceptés par le BMF pour un vendeur en Allemagne ; `Check` y ajoute l'`/AFRelationship` admise dans ce pays (voir plus bas) |
| `Rules: validate.Default \| validate.France` | Choix explicite des jeux de règles (`France`, `Germany`, `GermanyProfile`, `ChorusPro`, `Peppol`, `Netherlands`) ; les règles BR-DE d'une facture XRECHNUNG, les règles Peppol d'une facture Peppol et les règles BR-NL d'une facture NLCIUS s'y ajoutent toujours |
| `RejectBasicWL: true` | Avec les règles françaises, signale BASIC WL en erreur (FR-PROFILE-03) |
| `Rules: … \| validate.ChorusPro` | Ajoute les contrôles B2G de Chorus Pro (voir ci-dessous) |
| `Rules: … \| validate.Peppol` | Ajoute les règles Peppol BIS Billing 3.0 à une facture qui ne se déclare pas Peppol (voir ci-dessous) |
| `Rules: … \| validate.Netherlands` | Ajoute les règles néerlandaises de NLCIUS à une facture qui ne se déclare pas NLCIUS (voir ci-dessous) |
| `Profile: invoice.ProfileBasic` | Valide contre un profil plutôt que celui que déclare BT-24 |
| `Hybrid: true` | Le XML est celui d'un PDF Factur-X : une facture EN 16931 reçoit le schematron Factur-X de son profil plutôt que les règles CEN (voir [Factur-X ou CEN](#factur-x-ou-cen)) ; `zerechnung.Check` et `zerechnung.Create` le fixent, `zerechnung validate -hybrid` aussi |

`validate.Catalog(set)` liste les règles implémentées.

#### Profils admis par pays

Les schematrons officiels ne contrôlent pas le profil ; ces contrôles ont des identifiants propres au package.

| Règle | Gravité | Jeu | Constat |
|---|---|---|---|
| FR-PROFILE-01 | erreur | `France` | MINIMUM, exclu de la réforme française |
| FR-PROFILE-02 | avertissement | `France` | BASIC, que la réforme ne liste pas (absent du paquet FNFE-MPE et des capacités de l'annuaire) |
| FR-PROFILE-03 | erreur | `France` avec `RejectBasicWL` | BASIC WL, admis aujourd'hui sans date de fin publiée : l'option permet de le refuser dès maintenant ou quand il ne sera plus admis |
| DE-PROFILE-01 | erreur | `GermanyProfile` | MINIMUM et BASIC WL, que le BMF n'accepte pas comme factures électroniques (BMF-Schreiben du 15/10/2024 : ZUGFeRD à partir de 2.0.1, sauf ces deux profils) |

En France, EN 16931 et EXTENDED-CTC-FR forment le socle et EXTENDED est admis ; en Allemagne, BASIC, EN 16931, EXTENDED et XRECHNUNG sont acceptés. `Auto` n'applique pas les règles BR-DE à une facture ZUGFeRD : elles ne concernent que XRECHNUNG, à qui elles s'appliquent quels que soient les jeux choisis.

#### Leitweg-ID et GEBA

Le schematron XRechnung exige BT-10 (BR-DE-15) et admet les schémas d'adresse électronique 0204 (Leitweg-ID) et 0246 (GEBA) sans en contrôler la forme. Le jeu `Germany` vérifie aussi leur format et leur clé, en avertissement et avec des identifiants propres au package. Une valeur mal formée n'est pas soumise au contrôle de clé.

| Règle | Gravité | Valeurs contrôlées | Constat |
|---|---|---|---|
| DE-LEITWEG-01 | avertissement | BT-10 ; BT-34 et BT-49 de schéma 0204 | Format de la Formatspezifikation Leitweg-ID 2.0.2 : Grobadressierung de 2, 3, 5, 8, 9 ou 12 chiffres commençant par le code d'un Land (01 à 16) ou du Bund (99), Feinadressierung facultative de 1 à 30 lettres ou chiffres, deux chiffres de contrôle, séparés par « - » |
| DE-LEITWEG-02 | avertissement | idem | Chiffres de contrôle ISO/IEC 7064 MOD 97-10 ; le message donne les chiffres attendus |
| DE-GEBA-01 | avertissement | BT-34 et BT-49 de schéma 0246 | Format de la GEBA Formatspezifikation 1.0.1 : `DE` et les 9 chiffres de la W-IdNr, puis en option « - » et un Unterscheidungsmerkmal de 5 chiffres, puis en option « . » et une sous-adresse de 1 à 8 lettres ou chiffres |
| DE-GEBA-02 | avertissement | idem | Chiffre de contrôle de la W-IdNr (ISO 7064 MOD 11,10, comme pour la USt-IdNr) |

Dans une facture XRECHNUNG, BT-10 porte la Leitweg-ID de l'acheteur public : une facture B2B qui y met une autre référence reçoit DE-LEITWEG-01, qu'elle peut ignorer.

#### Relation du XML au PDF (`/AFRelationship`) par pays

La spécification Factur-X 1.09 (§ 5.3 et § 6.2.2) fixe la relation du XML au PDF selon le profil et le pays :

| Profil | France | Allemagne |
|---|---|---|
| MINIMUM, BASIC WL | `Data` | `Data` |
| BASIC, EN 16931, EXTENDED, EXTENDED-CTC-FR | `Alternative`, `Source` ou `Data` | `Alternative` |
| XRECHNUNG | — | `Alternative` |

- Par défaut, `Embed` et `Create` écrivent `Data` pour MINIMUM et BASIC WL, `Alternative` sinon : ces valeurs conviennent aux deux pays.
- `Embed` (et donc `Create`) refuse une valeur `EmbedOptions.Relationship` que le tableau exclut : autre chose que `Data` pour MINIMUM et BASIC WL, autre chose qu'`Alternative` pour XRECHNUNG ou pour un vendeur établi en Allemagne (BT-40 = `DE`).
- `Check` signale une valeur qui ne correspond pas au profil (PDF-14, avertissement). `Data` pour BASIC et au-delà ne l'est pas quand les règles françaises s'appliquent (`France`, ou `Auto` pour un vendeur en France), puisque la France l'admet.
- `Check` signale en erreur une autre valeur qu'`Alternative` pour BASIC et au-delà quand les règles allemandes s'appliquent (PDF-40) : profil XRECHNUNG, jeux `Germany` ou `GermanyProfile`, ou `Auto` pour un vendeur en Allemagne. `CheckContainer`, qui ne reçoit pas d'options, n'applique cette règle qu'à XRECHNUNG.

#### Factures B2G (Chorus Pro)

Le jeu `ChorusPro` contrôle une facture adressée à une structure publique, pour anticiper les rejets de Chorus Pro. Il n'est jamais appliqué d'office : rien dans le XML ne dit que l'acheteur est public. Les exigences qui dépendent du paramétrage de la structure dans l'annuaire Chorus Pro se donnent dans `PublicBuyer` :

```go
report := validate.XML(xml, &validate.Options{
	Rules:       validate.Default | validate.France | validate.ChorusPro,
	PublicBuyer: validate.PublicBuyer{ServiceCodeRequired: true, EngagementRequired: true},
})
```

| Règle | Contrôle | Rejet Chorus Pro évité |
|---|---|---|
| CPRO-01 | La structure publique est identifiée par son SIRET : BT-46 avec le schéma 0009 (réforme), ou BT-47 de 14 caractères avec le schéma 0002 ou sans schéma (format historique de Chorus Pro) | COD-021 |
| CPRO-02 | Ce SIRET a 14 chiffres et une clé de Luhn valide (pour La Poste, somme des chiffres multiple de 5) | — |
| CPRO-03 | Le code service (BT-10) est présent, si `ServiceCodeRequired` | COD-022 |
| CPRO-04 | Le code service a au plus 100 caractères : lettres, chiffres, espace, `_` et `-` | FOR-009, FOR-047 |
| CPRO-05 | Le numéro d'engagement (BT-13), ou à défaut la référence du marché (BT-12), est présent, si `EngagementRequired` | COH-053 |
| CPRO-06 | BT-13 et BT-12 ont au plus 50 caractères | FOR-013 |
| CPRO-07 | Le type de document (BT-3) n'est pas 751 | COD-001 |
| CPRO-08 | Le numéro de facture (BT-1) et celui de la facture d'origine (BT-25) ont au plus 20 caractères : lettres et chiffres ASCII, espace, `-`, `+`, `_` et `/` | FOR-049 |

Sources : annexe EDI des spécifications externes Chorus Pro V4.23 (règles G1.05, G2.06, G2.07, G2.16 à G2.19, G2.29, G3.01, G3.03, G3.04, S2.04, S2.13), annexe des messages d'erreur V3.02, dossier Chorus Pro des spécifications externes DGFiP v3.1 (numéro de facture limité à 20 caractères en B2G après la réforme). Ces identifiants sont propres au package : aucun schematron officiel ne couvre ces contrôles. Ce que Chorus Pro vérifie dans son annuaire (existence du SIRET et du code service, structure active) ne peut pas l'être hors ligne.

#### Peppol BIS Billing 3.0

Le jeu `Peppol` applique le schematron Peppol BIS Billing 3.0 de la syntaxe de la facture : les 90 assertions de `PEPPOL-EN16931-CII.sch` à une facture CII, les 165 de `PEPPOL-EN16931-UBL.sch` à une facture UBL, vérifiées sur le document UBL lui-même et non sur sa vue CII. Les deux ont en commun les règles PEPPOL-EN16931-R (processus métier BT-23, identifiant de spécification BT-24, référence acheteur ou de commande, adresses électroniques du vendeur et de l'acheteur, une seule note, totaux de TVA, remises et charges, montant net des lignes, périodes de ligne), formats des identifiants selon leur schéma ICD (PEPPOL-COMMON-R : GLN, numéros d'organisation norvégien, suédois, danois, belge, néerlandais, codes italiens, ABN australien…), listes de codes restreintes, et règles nationales norvégiennes, danoises, italiennes, suédoises et néerlandaises, appliquées selon le pays du vendeur. Le schematron UBL a en plus ses règles propres : pas d'élément vide (PEPPOL-EN16931-R008), une seule devise pour tous les montants (R051), remises sur le prix (R044, R046), codes de type de facture et d'avoir (P0100, P0101, P0112), motifs d'exonération de TVA (P0104 à P0111), et règles nationales grecques (GR-*), islandaises (IS-R-*) et allemandes (DE-R-*).

Il s'applique d'office quand BT-24 est l'identifiant Peppol (`urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0`, profil `invoice.ProfilePeppol`), quels que soient les jeux choisis dans `Rules`, sinon seulement sur demande (`Rules: validate.Default | validate.Peppol`, `zerechnung validate -peppol`). `Auto` ne l'ajoute jamais : la plupart des factures Factur-X et ZUGFeRD valides ne respectent pas ces règles (pas de BT-23 Peppol, pas d'adresse électronique, pas de référence acheteur), et les appliquer sans condition les signalerait à tort. En UBL, BT-24 est `cbc:CustomizationID`.

Les règles reproduisent chaque schematron tel qu'il est écrit. Le schematron CII vise parfois l'UBL plus que le CII :

- les identifiants de TVA sont cherchés avec le `schemeID` `VAT` de l'UBL : un identifiant CII (`VA`) ne sert ni à établir le pays du vendeur (c'est alors son adresse qui décide des règles norvégiennes et italiennes), ni aux contrôles NO-R-001, SE-R-001 et SE-R-002 ;
- PEPPOL-EN16931-P0100 porte sur `ram:ExchangedDocument/ram:TypeCode`, absent d'une facture CII (le document y est `rsm:ExchangedDocument`) : la règle ne se déclenche pas ;
- DK-R-007 cherche le mandat de prélèvement sous le moyen de paiement, alors qu'il figure dans les conditions de paiement en CII.

En UBL, le pays du vendeur des règles norvégiennes, italiennes et grecques est le préfixe de son identifiant de TVA (ou de celui du représentant fiscal), et son adresse seulement à défaut ; les règles danoises, islandaises, néerlandaises et allemandes regardent les adresses. Le schematron UBL a aussi ses écarts : PEPPOL-EN16931-R080 ne porte que sur un avoir, DK-R-005 à DK-R-011 que sur une facture, NL-R-001 que sur un `cbc:CreditNoteTypeCode` ; IS-R-009 et IS-R-010 cherchent la date d'échéance au niveau du document, où un avoir ne la porte pas ; DE-R-022 compare les noms de fichier en respectant la casse. DK-R-003 y est une erreur, alors que le schematron CII en fait un avertissement.

#### NLCIUS (Pays-Bas)

Le jeu `Netherlands` applique les règles de NLCIUS, la CIUS néerlandaise d'EN 16931, telles que l'autorité Peppol des Pays-Bas (NPa) les publie dans ses artefacts de validation (release du 2026-05-21, en vigueur le 17 août 2026). À une facture CII, il applique les 32 assertions du motif `nlcius` de `NLCIUS-CII-validation.sch` (NLCIUS-CII 1.0.3.13). À une facture UBL (SI-UBL 2.0), il applique les 35 assertions du motif `nlcius` de `si-ubl-2.0-nlcius.sch` et du motif `SI-UBL-VERSION` de `si-ubl-2.0.sch` (SI-UBL 2.0.3.13), vérifiées sur le document UBL lui-même. Les règles EN 16931 que ces validateurs incluent sont celles du jeu `EN16931`. Les règles BR-NL portent sur l'identifiant KVK ou OIN du vendeur et de l'acheteur néerlandais (BR-NL-1, BR-NL-10), la référence acheteur ou de commande (BR-NL-2, BR-NL-13), les adresses complètes (BR-NL-3 à BR-NL-5), les codes de type de document (BR-NL-7, et en UBL BR-NL-8 et BR-NL-9), le moyen de paiement (BR-NL-11, BR-NL-12) ; une vingtaine d'avertissements déconseillent des données sans objet aux Pays-Bas (BR-NL-19 à BR-NL-35) et les éléments vides.

Il s'applique d'office quand BT-24 est l'identifiant NLCIUS (`urn:cen.eu:en16931:2017#compliant#urn:fdc:nen.nl:nlcius:v1.0`, profil `invoice.ProfileNLCIUS`), seul ou suivi de l'extension G-rekening (`#conformant#urn:fdc:nen.nl:gaccount:v1.0`), quels que soient les jeux choisis dans `Rules`, sinon seulement sur demande (`Rules: validate.Default | validate.Netherlands`, `zerechnung validate -nl`). Comme pour Peppol, `Auto` ne l'ajoute jamais : un vendeur établi aux Pays-Bas n'est pas tenu d'émettre en NLCIUS. Les schematrons choisissent eux-mêmes les factures qu'ils visent, et le package fait de même : la plupart des règles ne valent que pour un fournisseur établi aux Pays-Bas (BT-40), BR-NL-13 (et BR-NL-7 en CII) pour toute facture NLCIUS. Sur une facture UBL qui ne se déclare pas NLCIUS, le jeu demandé signale SI-V20-INV-R000, comme le validateur SI-UBL.

Les règles reproduisent chaque schematron tel qu'il est écrit, ordre des règles compris : un élément n'est vérifié que par la première règle dont le contexte lui correspond. Quelques assertions des artefacts ne se déclenchent donc jamais et ne sont pas écrites : BR-NL-9 et BR-NL-31 en CII, BR-NL-32-2, BR-NL-32-3 et les assertions au message de BR-NL-34 en UBL (un code motif de remise ou de charge, au niveau du document comme de la ligne, donne BR-NL-32-1). Les deux schematrons diffèrent aussi entre eux : en CII, BR-NL-1 ne s'applique que si le vendeur a une `ram:SpecifiedLegalOrganization`, BR-NL-10 exige un identifiant légal de l'acheteur quel que soit son pays, et les codes de type sont comparés comme des nombres (`0380` vaut 380) ; en UBL, BR-NL-11 ne vise qu'une facture, pas un avoir. L'extension G-rekening (`si-ubl-2.0-ext-gaccount.sch`) n'est pas écrite.

### En ligne de commande

```bash
go install github.com/ritoon/zerechnung/cmd/zerechnung@latest
```

```text
zerechnung extract [-o out.xml] facture.pdf
zerechnung info facture.pdf
zerechnung validate [-fr] [-de] [-peppol] [-nl] [-auto] [-reject-basicwl] [-choruspro [-service-required] [-engagement-required]] [-profile p] [-hybrid] [-json] facture.pdf|facture.xml
zerechnung embed -xml facture.xml [-profile p] [-attach fichier]... [-o out.pdf] entree.pdf
zerechnung json facture.pdf|facture.xml
zerechnung rules [-fr] [-de] [-choruspro] [-peppol] [-nl]
```

`validate` et `json` acceptent un XML CII ou UBL. `validate` sort avec le code 1 quand la facture a des erreurs, ce qui permet de l'utiliser en intégration continue. Ses options de règles (`-fr`, `-de`, `-choruspro`, `-peppol`, `-nl`) s'ajoutent aux règles par défaut : elles ne retirent jamais les règles BR-DE d'une facture XRECHNUNG, les règles Peppol d'une facture Peppol ni les règles BR-NL d'une facture NLCIUS. `embed -attach` ajoute une pièce jointe complémentaire (`/AFRelationship /Supplement`, type MIME d'après l'extension) ; l'option se répète.

```text
$ zerechnung validate testdata/EN16931_Einfach.pdf
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
| XRECHNUNG (CII) | oui | oui | schéma CII D16B complet, comme le validateur KoSIT | oui ; BR-DE appliquées d'office |
| EXTENDED-CTC-FR | oui | oui | schéma EXTENDED, inclus dans le schéma CII D22B du paquet FNFE-MPE | oui ; en CII, règles de syntaxe du schematron EXTENDED-CTC-FR de la FNFE-MPE ; BR-FR sur option (`France` ou `Auto`) |
| PEPPOL (CII) | oui | oui | schéma EN 16931 | oui ; règles Peppol appliquées d'office |
| NLCIUS (CII) | oui | oui | schéma EN 16931 | oui ; règles BR-NL appliquées d'office |
| ZUGFeRD 1.0 (BASIC, COMFORT, EXTENDED) | oui (BASIC, EN 16931, EXTENDED) | non : réécrit en Factur-X | contrôle structurel minimal (ZF1-01 à ZF1-03) | sans objet : antérieur à EN 16931 (avertissement ZF1-04) |
| UBL 2.1 (EN 16931, XRECHNUNG, PEPPOL, NLCIUS) | oui | oui | schéma UBL 2.1 (Invoice, CreditNote), sauf le contenu des extensions | sans objet ; BR-DE (schematron UBL) appliquées d'office pour XRECHNUNG, règles Peppol (schematron UBL) pour PEPPOL, BR-NL (schematron SI-UBL 2.0) pour NLCIUS, BR-FR (schematron UBL) sur option |

Une facture XRECHNUNG en CII est contrôlée, comme dans le validateur KoSIT, contre le schéma UN/CEFACT CII D16B complet de sa configuration (XRechnung 3.0.2) : les éléments D16B hors du jeu EN 16931 y sont admis et ce sont les règles CII-SR qui les signalent ; une seule référence à une facture antérieure (BG-3) y est permise, et `cii.Marshal` n'en écrit donc qu'une pour XRECHNUNG. Une facture EXTENDED-CTC-FR en CII reçoit, à la place des règles de syntaxe du CEN, celles du schematron `EXTENDED-CTC-FR-CII.sch` de la FNFE-MPE (France_RFE 1.4.0.04), avec ses niveaux et ses messages : 434 CII-SR (38 bloquantes), 92 CII-DT et 9 règles propres (CII-FREXT-SR, CII-EXT-DT-018, CII-FREXT-DT-024, CII-FREXTDT-027). Les données qu'EXTENDED-CTC-FR admet n'y sont pas signalées : notes de ligne avec sujet, type et mesure d'une caractéristique d'article, référence de commande et Incoterm à la ligne, motif d'exonération sur une remise ou une charge, virement sans compte, plusieurs cartes ou mandats de paiement, pièce jointe sans type MIME ni nom de fichier, type de la référence du contrat…

Elle reste contrôlée contre le schéma EXTENDED, inclus dans le schéma CII D22B avec lequel le paquet FNFE-MPE valide les factures CII (les tests le vérifient), et pas contre le D22B. La FNFE complète en effet le D22B par tout son schematron, y compris un motif métier (BR-FREXT…) qui n'est pas écrit ici : le D22B et les règles de syntaxe ne rendraient donc pas son verdict complet. Le schéma EXTENDED est plus strict que la FNFE, dont le paquet ne connaît d'ailleurs EXTENDED-CTC-FR qu'en CII et en UBL, pas en Factur-X. Sur 2 931 variantes des 5 exemples EXTENDED-CTC-FR de France_RFE (suppression ou duplication de chaque élément, mutations ciblées sur les règles de syntaxe), 125 sont acceptées par la FNFE (valides contre le D22B, aucune règle bloquante) mais rejetées par le schéma EXTENDED. Ce sont deux contenus dans une note, deux descriptions d'article ou de conditions de paiement, un second `BusinessProcessSpecifiedDocumentContextParameter`, une description dans les conditions de livraison, et même l'absence des totaux du document (BG-22), que le schematron de la FNFE ne bloque pas.

### Règles

| Jeu | Règles | Référence |
|---|---|---|
| Schéma | 11 contrôles (éléments, ordre, cardinalités, attributs, espaces de noms, types, listes fermées, valeurs fixes) | XSD Factur-X 1.09.2 (ZUGFeRD 2.5.2), XSD UN/CEFACT CII D16B de la configuration KoSIT XRechnung 3.0.2 (XRECHNUNG) et XSD OASIS UBL 2.1 (Invoice, CreditNote), compilés en tables Go |
| EN 16931 | 793 assertions (328 erreurs, 465 avertissements) : BR, BR-CO, BR-CL, BR-DEC, règles par catégorie de TVA (BR-S, BR-Z, BR-E, BR-AE, BR-IC, BR-IG, BR-IP, BR-G, BR-O, BR-AF, BR-AG, BR-B), CII-SR, CII-DT ; pour EXTENDED-CTC-FR en CII, 535 règles de syntaxe (108 erreurs, 427 avertissements) à la place de celles du CEN : CII-SR (434), CII-DT (92), CII-FREXT et CII-EXT (9) | Artefacts CEN TC 434 v1.3.17 (CII) ; motif de syntaxe du schematron EXTENDED-CTC-FR-CII FNFE-MPE v1.4.0.04 |
| EN 16931 (UBL) | Mêmes règles métier, avec les contextes et les tests des artefacts UBL (catégories L et M, tolérances, BR-DEC sur le texte, listes de codes propres à l'UBL) ; 759 règles de syntaxe (62 erreurs, 697 avertissements) : UBL-CR (678), UBL-SR (57), UBL-DT (24) | Artefacts CEN TC 434 v1.3.17 (UBL) |
| Factur-X | 544 règles (523 erreurs, 21 avertissements) : identification du profil (BT-24), assertions des schematrons MINIMUM et BASIC WL, éléments non utilisés par profil ; listes de codes des schematrons BASIC (48), EN 16931 (58) et EXTENDED (209) ; règles BR-FXEXT d'EXTENDED et EXTENDED-CTC-FR (sous-types et regroupements de lignes, totaux avec tolérance, frais logistiques et frais pour compte de tiers, ventilation de TVA par catégorie, taux et motif d'exonération) | Schematrons Factur-X 1.09 (MINIMUM, BASIC WL) ; schematrons et listes de codes Factur-X 1.09.2 (BASIC, EN 16931, EXTENDED) ; XSLT EXTENDED Factur-X 1.09.2 (ZUGFeRD 2.5.2) |
| France (BR-FR) | 59 règles, 170 assertions en CII, 173 en UBL : SIREN/SIRET, adresses électroniques, mentions obligatoires, cadres de facturation, multi-vendeurs ; 3 contrôles des profils admis (FR-PROFILE) | Schematrons FNFE-MPE v1.4.0.04 (CII et UBL) |
| Allemagne (BR-DE) | 52 assertions en CII, 57 en UBL (BR-DE, BR-DE-CVD, BR-DEX, BR-TMP ; PEPPOL-EN16931-R043 en UBL) ; contrôle des profils acceptés par le BMF (DE-PROFILE-01) ; format et clé de la Leitweg-ID et de la GEBA (DE-LEITWEG-01, DE-LEITWEG-02, DE-GEBA-01, DE-GEBA-02) | Schematrons KoSIT XRechnung 3.0.2 (CII et UBL) ; BMF ; Formatspezifikation Leitweg-ID 2.0.2 ; GEBA Formatspezifikation 1.0.1 |
| Peppol BIS Billing 3.0 | CII : 90 assertions (73 erreurs, 17 avertissements) ; UBL : 165 assertions (139 erreurs, 26 avertissements), dont 76 propres à l'UBL. PEPPOL-EN16931-R, PEPPOL-COMMON-R (identifiants par schéma ICD), listes de codes restreintes, règles nationales NO, DK, IT, SE et NL, et en UBL GR, IS et DE | Schematrons Peppol BIS Billing 3.0.21 (`PEPPOL-EN16931-CII.sch` et `PEPPOL-EN16931-UBL.sch`, inchangés en 3.0.22) |
| Pays-Bas (BR-NL) | CII : 32 assertions (10 erreurs, 22 avertissements) ; UBL : 35 assertions (14 erreurs, 21 avertissements), dont le contrôle de version SI-UBL 2.0 (BII2-T10-R001, SI-V20-INV-R000). Identifiants KVK et OIN, références acheteur et de commande, adresses, types de document, moyens de paiement, données déconseillées, éléments vides | Artefacts NPa du 2026-05-21 : NLCIUS-CII 1.0.3.13 (`NLCIUS-CII-validation.sch`), SI-UBL 2.0.3.13 (`si-ubl-2.0.sch`, `si-ubl-2.0-nlcius.sch`) |
| ZUGFeRD 1.0 | 4 contrôles (ZF1-01 à ZF1-04) : identifiant de profil ZUGFeRD 1.0, valeurs lisibles, données qui identifient une facture ; avertissement : ni règles EN 16931, ni Factur-X, ni nationales | Spécification ZUGFeRD 1.0 (FeRD, 2014) |
| Chorus Pro (B2G) | 8 contrôles (CPRO-01 à CPRO-08) : SIRET de la structure publique, code service, numéro d'engagement, type de document, numéro de facture | Spécifications externes Chorus Pro (annexe EDI V4.23) |
| Conteneur PDF | 40 contrôles (PDF-01 à PDF-40) : PDF/A-3 ou PDF/A-4f déclaré, cohérent avec la version PDF, XMP `fx` cohérent avec le XML, pièce jointe, `/AF`, `/AFRelationship` selon le profil et le pays, type MIME, chiffrement ; prérequis PDF/A vérifiables sans rendu : polices incorporées, JavaScript et actions interdites (`/OpenAction`, liens, signets, `/AA`), cohérence entre `/Info` et le XMP, pièces jointes annexes (`/AFRelationship`, type MIME, présence dans un `/AF`), filtre `LZWDecode` | Spécification Factur-X 1.08 et 1.09 (§ 6.2.2), ISO 19005-3 et 19005-4 |

Les messages sont ceux des artefacts officiels. Les listes de codes (pays, devises, unités, schémas d'identifiants, motifs d'exonération…) sont embarquées.

#### Factur-X ou CEN

Une facture BASIC ou EXTENDED reçoit le schematron Factur-X 1.09.2 de son profil (FNFE-MPE / FeRD), sa référence officielle, que Mustang applique aussi. Il reprend les règles métier du CEN, mais vérifie les listes de codes avec ses propres listes (règles FX-SCH-A, à la place des BR-CL du CEN), ne garde qu'une partie des règles de syntaxe CII, met CII-SR-465 et CII-SR-466 (contact avec un nom de personne et un service) en erreur et refuse un BT-24 d'orthographe obsolète (FX-PROFILE-03 en erreur). Les deux jeux de listes diffèrent sur quelques codes : le schéma d'adresse électronique 9958, les motifs de remise ABK, ADZ et CAO, les unités d'emballage (`PackageQuantity`) d'EXTENDED…

Une facture EN 16931 reçoit ce schematron quand elle est le XML d'un PDF Factur-X : `zerechnung.Check` et `zerechnung.Create` fixent `validate.Options.Hybrid`, que `zerechnung validate -hybrid` fixe aussi pour un XML seul. Sans cela, elle reçoit les règles CEN 1.3.17, comme dans le validateur KoSIT : son identifiant, `urn:cen.eu:en16931:2017`, est celui de toute facture CII conforme à EN 16931 sans CIUS, et rien dans le XML ne dit qu'il est destiné à un PDF Factur-X. Les verdicts ne diffèrent alors que sur les listes de codes, sur l'attribut `currencyID` d'un montant autre que le total de TVA (CII-DT-031, une erreur pour le CEN, admis par Factur-X) et sur le niveau de CII-SR-465 et CII-SR-466.

### Exactitude mesurée

Le workflow [Conformité](.github/workflows/conformance.yml) exécute à chaque pull request les validateurs de référence sur 282 factures du corpus ZUGFeRD et de la suite de tests KoSIT, et compare facture par facture les règles levées en erreur (méthode et détail dans [conformance](conformance/README.md#parité-avec-kosit-et-mustang)) :

| Référence | Factures comparées | Mêmes erreurs | Écarts justifiés |
|---|---|---|---|
| KoSIT validator 1.6.3, configuration XRechnung 3.0.2 du 2026-08-31 | 128 | 128 | 0 |
| Mustang 2.26.0 | 282 | 241 | 41 |

Avec KoSIT, toutes les factures comparées ont les mêmes erreurs. Avec Mustang, chaque écart est justifié dans [conformance/parity/known.txt](conformance/parity/known.txt), et tout nouvel écart fait échouer le job ; ils viennent des références elles-mêmes : Mustang n'applique ni Peppol BIS 3.0, ni la configuration KoSIT aux factures XRechnung (niveaux des règles, XSD CII D16B), et embarque le schematron XRechnung 2.4.0 (2.6.0 pour KoSIT et le package). Les 26 factures invalides à dessein du corpus (`ZUGFeRDv2/fail`, exemples `not_validating`) sont vérifiées une à une par `TestOfficialNegativeCases` contre les erreurs des références.

Les 104 PDF hybrides du corpus (`ZUGFeRDv2/correct`, `XML-Rechnung/FX`) sont tous extraits et leurs 26 XML tous lus.

Les 73 factures UBL du corpus (`XML-Rechnung/UBL`, 28) et de la suite de tests KoSIT (45) sont lues, réécrites puis relues dans le même modèle, et validées sans autre erreur que les 3 attendues : BR-CL-25 sur les deux exemples au schéma 9958, comme en CII, et l'exemple `not_validating…`, invalide à dessein. Toutes respectent le schéma UBL 2.1. Sur 11 000 variantes de ces factures modifiées au hasard (ordre, cardinalités, types, attributs, espaces de noms, extensions), le contrôle du schéma rend le même verdict que xmllint (libxml2) avec les XSD OASIS.

Les règles de syntaxe d'EXTENDED-CTC-FR rendent les mêmes constats CII (même nombre par règle et par niveau) que la XSLT `EXTENDED-CTC-FR-CII.xslt` de France_RFE 1.4.0.04 exécutée par Saxon. C'est vrai sur les 28 XML CII et Factur-X des exemples, qui n'en ont aucun, et sur 10 000 variantes des 5 exemples EXTENDED-CTC-FR modifiées chacune pour enfreindre une règle de syntaxe : constats identiques sur les 8 716 que la XSLT traite, alors qu'elle s'arrête sur une erreur de type pour les 1 284 autres (montants répétés notamment). 525 des 535 règles y sont déclenchées ; les tests unitaires couvrent CII-SR-038 et CII-SR-339, ainsi que des règles de comptage des totaux (CII-SR-477 à 487) sur lesquelles la XSLT s'arrête.

Les 41 factures CII de la suite de tests KoSIT respectent le schéma CII D16B, comme dans le validateur KoSIT, et les 28 factures CII et Factur-X des exemples FNFE-MPE (France_RFE 1.4.0.04, `Z.example/TEST`) le schéma de leur profil ainsi que le schéma CII D22B. Sur 9 000 variantes modifiées au hasard (3 000 factures KoSIT contrôlées contre le D16B, 3 000 factures EXTENDED contre le D22B et les mêmes contre le XSD EXTENDED), le contrôle du schéma rend le même verdict que xmllint (libxml2) avec les XSD d'origine. Les tests vérifient enfin, sur les tables compilées, que chaque schéma Factur-X est inclus dans le schéma de référence de son profil : EN 16931 dans le D16B, au nombre de BG-3 près, et EXTENDED dans le D22B.

Les règles Peppol donnent le résultat attendu sur les 351 tests unitaires CII du dépôt Peppol BIS Billing 3.0 (87 jeux de tests, dossiers `rules/unit-CII-*`, versions 3.0.21 et 3.0.22) ; les trois assertions qu'ils ne couvrent pas (PEPPOL-COMMON-R040, R041, R056-2) le sont par les tests du package. Les règles du schematron UBL donnent le résultat attendu sur les 678 tests unitaires UBL (153 jeux de tests, 687 assertions, dossiers `rules/unit-UBL-*`, versions 3.0.21 et 3.0.22) ; les 13 assertions qu'ils ne font jamais échouer (PEPPOL-EN16931-R008, PEPPOL-COMMON-R041, R056-2, IS-R-001 à IS-R-010) le sont par les tests du package. Les 26 exemples officiels (`rules/examples`, `rules/snippets`) passent la validation par défaut (schéma UBL, EN 16931 et Peppol) sans erreur.

Le catalogue des règles BR-NL reprend, identifiant par identifiant, le niveau et le message de chaque assertion des schematrons de la NPa, ce que vérifie un test. Sur les 97 fichiers de test de SimplerInvoicing pour SI-UBL 2.0.3.2 et NLCIUS-CII 1.0.3 (repris par phive-rules), dont le nom dit la règle enfreinte et son niveau, le package lève cette règle avec ce niveau, et rien d'autre. Seuls 4 noms ne disent pas toute la règle levée : le code 381 sur une `Invoice`, que BR-NL-8 signale depuis SI-UBL 2.0.3 ; le code 532 sur un avoir, qui enfreint aussi BR-NL-7 ; les deux fichiers au message de BR-NL-34, dont les assertions portent l'identifiant BR-NL-32-1. Les 37 fichiers qui n'enfreignent aucune règle passent la validation complète (schéma, EN 16931, BR-NL) sans erreur. Les règles BR-NL ne sont pas confrontées à une exécution des schematrons officiels, faute de validateur de référence dans le job de parité.

Sur les 147 XML CII lus du corpus ZUGFeRD et des XML extraits de ses PDF, 145 sont réécrits en EXTENDED avec tout leur arbre ; les 2 autres portent des données qu'aucun profil Factur-X n'admet (le code banque allemand de ZUGFeRD 2.0, retiré depuis du schéma, et un type de référence sur un document justificatif autre que 130), et `Marshal` les signale. Un document qui contient chaque élément du XSD EXTENDED avec tous ses attributs est relu et réécrit à l'identique, sauf ce même élément interdit par les règles.

Les 21 XML ZUGFeRD 1.0 extraits des PDF de `ZUGFeRDv1/correct` sont tous lus, l'un avec une date illisible (`20139102`, une erreur du document). Réécrits en Factur-X, 8 gardent tout leur arbre en EXTENDED ; les 12 autres lisibles ne perdent que des données qu'aucun profil Factur-X n'admet (nom de l'établissement bancaire, code banque allemand, taxe d'assurance, immatriculation fiscale vide…), et `Marshal` les signale. Les 3 PDF de `ZUGFeRDv1/fail` ont chacun au moins une erreur ZF1-01 à ZF1-03.

### PDF

Le lecteur PDF est écrit pour ce package : tables xref classiques et flux xref, flux d'objets, mises à jour incrémentales, reconstruction quand la table xref est endommagée, filtres Flate (avec prédicteurs), ASCIIHex et ASCII85. La décompression est bornée (256 Mio par flux) et la profondeur d'imbrication limitée, pour qu'un fichier hostile ne puisse ni saturer la mémoire ni faire déborder la pile. Le décompresseur est testé par fuzzing différentiel contre `compress/flate`.

Le lecteur XML est lui aussi écrit pour ce package : un seul passage sur les octets, espaces de noms résolus, imbrication limitée à 256 niveaux, déclarations DOCTYPE ignorées et aucune entité développée hors des cinq prédéfinies. Il est testé par fuzzing différentiel contre `encoding/xml` : mêmes documents acceptés, même arbre. La lecture rapide de BT-24 (le profil de la pièce jointe, lu par `Embed` et `CheckContainer` sans construire d'arbre) est à son tour comparée à ce lecteur par fuzzing : même élément racine, même identifiant.

## Limites

- **Pas de conversion PDF/A.** `Embed` déclare PDF/A-3 (ou PDF/A-4f pour un PDF 2.0) dans le XMP mais ne touche ni aux polices, ni aux espaces de couleur, ni aux output intents : le PDF hôte doit déjà respecter PDF/A-2, PDF/A-3 ou PDF/A-4 (polices incorporées, output intent), ce que la plupart des générateurs savent faire sur option. `Check` signale ce qui manque parmi ce qui se vérifie sans rendu (output intent, polices non incorporées, JavaScript et actions interdites, `/Info` incohérent avec le XMP, filtre LZW) ; il ne remplace pas un validateur PDF/A comme veraPDF : les espaces de couleur, la transparence, les programmes de police eux-mêmes et le contenu des pages ne sont pas analysés. Une police présente dans les ressources d'une page sans être incorporée est un avertissement (PDF-31), le contenu n'étant pas lu pour savoir si elle sert à afficher du texte visible.
- **UBL : extensions non vérifiées, BR-FR non confrontées aux schematrons officiels.** Le schéma UBL 2.1 déclare le contenu des extensions (`ext:ExtensionContent`) en `xsd:any` lax, et les schémas de signature qu'il importe pour cela (xmldsig, XAdES) ne sont pas compilés : seuls l'espace de noms et le nombre des éléments d'une extension sont vérifiés, pas leur contenu. Les règles BR-FR suivent le schematron UBL de leur jeu, mais n'ont pas été confrontées à une exécution du schematron officiel : elles sont vérifiées par des tests de mutation. Les règles BR-DE, elles, sont confrontées à KoSIT par le job de parité, sur les factures UBL XRechnung du corpus et de la suite KoSIT (voir [Exactitude mesurée](#exactitude-mesurée)). Le jeu Allemagne n'écrit pas les règles Peppol que les schematrons XRechnung incorporent, sauf PEPPOL-EN16931-R043 en UBL : elles relèvent du jeu Peppol, et une facture UBL vérifiée avec les deux jeux voit R043 signalée par chacun. Les règles Chorus Pro tournent sur une vue CII du document ; leurs constats sont situés dans l'UBL.
- **NLCIUS : sans l'extension G-rekening ni parité exécutée.** Les 9 assertions de `si-ubl-2.0-ext-gaccount.sch` (compte G, deux moyens et deux conditions de paiement) ne sont pas écrites : une facture qui déclare l'extension reçoit les seules règles NLCIUS. Les règles BR-NL sont vérifiées par des tests de mutation, par les fichiers de test de SimplerInvoicing et par la comparaison de leur catalogue aux schematrons, mais aucun validateur de référence (KoSIT avec la configuration de la NPa, phive) ne les exécute en CI.
- **EXTENDED-CTC-FR : syntaxe CII seulement.** Le motif métier du schematron EXTENDED-CTC-FR de la FNFE (BR-FREXT, listes de codes, BR-CO-25 bloquante…) n'est pas appliqué : une facture EXTENDED-CTC-FR reçoit les règles métier de la XSLT Factur-X EXTENDED (BR-FXEXT et le sous-ensemble du CEN qu'elle reprend). En UBL, une facture EXTENDED-CTC-FR reçoit les règles UBL du CEN, pas celles du schematron UBL EXTENDED-CTC-FR.
- **PDF chiffrés** : signalés (PDF-01), pas déchiffrés.
- **ZUGFeRD 1.0 : lecture seule, contrôle minimal.** Le XML est lu et se réécrit en Factur-X, mais ne s'écrit pas en ZUGFeRD 1.0 ; il n'est vérifié ni contre le XSD ZUGFeRD 1.0, ni contre des règles métier (voir [ZUGFeRD 1.0](#zugferd-10)).
- Les règles des schematrons sont réécrites en Go, pas interprétées : une nouvelle version des artefacts officiels demande une mise à jour du code (les catalogues EN 16931 du CII et de l'UBL et celui d'EXTENDED-CTC-FR sont régénérés par `go generate`, les tests vérifient l'alignement).

## Performances

Apple M4 Pro, Go 1.26, 16/11/2026. Comparaison avec le plus rapide des autres packages Go qui réussit l'opération ; tableaux complets dans [bench/ecosystem/results/2026-11-16/tables.md](bench/ecosystem/results/2026-11-16/tables.md). Le contrôle complet a été remesuré le 17/11/2026 (Go 1.26.4), après l'ajout des contrôles PDF/A du conteneur (polices, JavaScript et actions, `/Info`, LZW).

| Opération | ZeRechnung | Meilleur autre package Go |
|---|---|---|
| Extraction du XML (PDF de 149 Ko) | 28 µs | 75 µs (speedata) |
| Lecture XML → modèle (13 Ko) | 15 µs | 126 µs (fX) |
| Lecture et validation complète (13 Ko) | 23 µs | 132 µs (pdf0 + formalis, sans règles françaises) |
| Lecture et validation complète (1,3 Mo, pièces jointes base64) | 744 µs | 7,01 ms (angelo) |
| Écriture modèle → XML (13 Ko) | 2,7 µs | 20,7 µs (fX) |
| Injection dans un PDF à flux xref | 80 µs | 543 µs (pdfcpu) |
| Contrôle complet d'un PDF hybride (149 Ko) | 68 µs | 1,18 ms (DCNWS) |

Le harnais est dans [bench/ecosystem](bench/ecosystem) ; chaque librairie y a son propre module.

La comparaison avec 32 librairies TypeScript, Python et Rust, sur le même corpus et avec le même protocole, est dans [bench/polyglot](bench/polyglot) : temps de réponse, exactitude, et normes réellement appliquées par chacune.

## Organisation

| Package | Rôle |
|---|---|
| `zerechnung` (racine) | Création, lecture, extraction, injection et contrôle d'un PDF hybride |
| `invoice` | Modèle EN 16931 (BT, BG), décimaux exacts, dates, profils, calcul des totaux |
| `cii` | Lecture et écriture du XML CII, profil par profil |
| `ubl` | Lecture et écriture du XML UBL 2.1 (Invoice, CreditNote) |
| `validate` | Validation : schéma, règles EN 16931 (CII et UBL), règles de profil Factur-X, règles BR-FR, BR-DE, Peppol et BR-NL |
| `cmd/zerechnung` | Outil en ligne de commande |
| `internal/…` | Lecteur et écrivain PDF, XMP, arbre XML, règles, listes de codes, tables de schéma |
| `bench/ecosystem` | Benchmarks comparatifs avec les autres librairies Go (modules séparés) |
| `bench/polyglot` | Benchmarks comparatifs avec les librairies TypeScript, Python et Rust |

## Tests

```bash
go test ./...
```

Les tests de base n'ont besoin que du dépôt ; la CI ([.github/workflows/ci.yml](.github/workflows/ci.yml)) les exécute avec `-race` sur Go 1.24 et sur la dernière version, après `gofmt` et `go vet`, et fait tourner 30 secondes de fuzzing sur le lecteur PDF, le codec Flate, le lecteur XML et la lecture de BT-24 dans la pièce jointe. Des tests supplémentaires s'activent quand les artefacts officiels sont disponibles. Le workflow [Conformité](.github/workflows/conformance.yml) les récupère à des versions figées (commit pour les dépôts, SHA-256 pour les fichiers) et les exécute tous à chaque pull request, en échouant si l'un d'eux est ignoré ; en local, `eval "$(conformance/fetch.sh)"` récupère les mêmes artefacts et définit les variables (voir [conformance](conformance/README.md)) :

| Variable | Contenu | Tests |
|---|---|---|
| `FX_CORPUS` | Clone de [ZUGFeRD/corpus](https://github.com/ZUGFeRD/corpus) | Extraction et contrôle des PDF du corpus ; lecture, aller-retour, schéma UBL 2.1 et validation des factures UBL de `XML-Rechnung/UBL` |
| `FX_CORPUS_XML` | XML extraits du corpus | Lecture, validation et aller-retour lecture → écriture : sans perte signalée, l'arbre complet du document d'origine doit se retrouver dans le XML réécrit, dans son profil et en EXTENDED ; un XML ZUGFeRD 1.0 (extrait d'un PDF de `ZUGFeRDv1`) est comparé à sa vue CII D16B |
| `FX_XSD` | XSD Factur-X 1.09.2, un dossier par profil (`MINIMUM`, `BASICWL`, `BASIC`, `EN16931`, `EXTENDED`) | Le XML écrit par `cii.Marshal` est vérifié contre les XSD d'origine |
| `FX_XSD_DIR` | XSD Factur-X 1.09.2 : paquet FeRD ZUGFeRD 2.5.2 (BASIC, EN 16931, EXTENDED) et paquet FNFE-MPE 1.09 (MINIMUM, BASIC WL, que le corrigendum 1.09.2 ne modifie pas) | Les tables de schéma régénérées sont identiques à celles du dépôt |
| `FX_KOSIT` | Clone de [itplr-kosit/xrechnung-testsuite](https://github.com/itplr-kosit/xrechnung-testsuite) | Toutes les factures CII et UBL de la suite de tests KoSIT passent les règles BR-DE ; ses factures CII respectent le schéma CII D16B ; ses factures UBL sont aussi lues, réécrites, vérifiées contre le schéma UBL 2.1 et validées sans erreur |
| `FX_KOSIT_CONF` | Archive de la [configuration du validateur KoSIT](https://github.com/itplr-kosit/validator-configuration-xrechnung/releases/tag/v2026-08-31) XRechnung 3.0.2, release 2026-08-31 | Les tables du schéma CII D16B régénérées sont identiques à celles du dépôt (`go generate -run D16B` dans `internal/schema` télécharge l'archive et vérifie son SHA-256 et celui de chaque XSD) |
| `FX_FNFE` | Clone de [fnfempe/France_RFE](https://github.com/fnfempe/France_RFE), tag v1.4.0.04 | Les exemples CII et Factur-X (`Z.example/TEST`) respectent le schéma de leur profil et le schéma CII D22B ; contrôlés en EXTENDED-CTC-FR, ils n'enfreignent aucune règle de syntaxe de son schematron ; les tables du D22B, compilées pour les tests seulement, et le catalogue `ctcfr_gen.go` (`go generate` dans `internal/rules/en16931` télécharge le schematron et vérifie son SHA-256) sont identiques à ceux du dépôt |
| `FX_PEPPOL` | Clone de [OpenPEPPOL/peppol-bis-invoice-3](https://github.com/OpenPEPPOL/peppol-bis-invoice-3), version 3.0.21 ou suivante | Les 351 tests unitaires CII (`rules/unit-CII-*`) et les 678 tests unitaires UBL (`rules/unit-UBL-*`) donnent le résultat attendu par le schematron ; les exemples officiels (`rules/examples`, `rules/snippets`) passent la validation sans erreur |
| `FX_NL` | Clone de [peppolautoriteit-nl/validation](https://github.com/peppolautoriteit-nl/validation), release 2026-05-21 | Chaque assertion des motifs NLCIUS (CII) et SI-UBL 2.0 (UBL) écrits figure dans le catalogue de sa syntaxe avec son niveau et son message, sauf celles qui ne se déclenchent jamais |
| `FX_NL_TESTS` | Dossier `simplerinvoicing` des fichiers de test de [phax/phive-rules](https://github.com/phax/phive-rules) | Les fichiers SI-UBL 2.0.3.2 et NLCIUS-CII 1.0.3 lèvent la règle BR-NL que dit leur nom, avec son niveau ; ceux qui n'en enfreignent aucune passent la validation complète sans erreur |
| `FX_UBL_ZIP` | Paquet OASIS [UBL-2.1.zip](https://docs.oasis-open.org/ubl/os-UBL-2.1/UBL-2.1.zip) | Les tables du schéma UBL régénérées sont identiques à celles du dépôt (`go generate -run -ubl` dans `internal/schema` télécharge le paquet et vérifie son SHA-256) |
| `FX_FACTURX_SCH` | Schematrons Factur-X 1.09.2 de BASIC, EN 16931 et EXTENDED et leurs listes de codes (`FACTUR-X_<profil>.sch` et `FACTUR-X_<profil>_codedb.xml`) | Les assertions de listes de codes régénérées (`codedb_gen.go`) sont identiques à celles du dépôt (`go generate` dans `internal/rules/facturx` télécharge les fichiers et vérifie leur SHA-256) |

## Licence et provenance

Le code est sous licence MIT (voir [LICENSE](LICENSE)).

Les règles sont réécrites en Go à partir des artefacts officiels, qui ne sont pas inclus dans le dépôt (les schémas XSD sont compilés en tables) ; leurs identifiants et leurs messages sont repris pour que les rapports correspondent aux validateurs de référence :

| Artefact | Version | Licence |
|---|---|---|
| Artefacts de validation EN 16931 pour CII et UBL (CEN TC 434, ConnectingEurope/eInvoicing-EN16931) | 1.3.17 | EUPL-1.2 |
| Schematron BR-FR (FNFE-MPE, fnfempe/France_RFE) | 1.4.0.04 | EUPL-1.2 |
| Schematron EXTENDED-CTC-FR-CII (FNFE-MPE, fnfempe/France_RFE), motif de syntaxe | 1.4.0.04 | EUPL (dérivé du schematron EN 16931, avis en tête du fichier) ; dépôt France_RFE sous Apache-2.0 |
| Schematron XRechnung (KoSIT) | 3.0.2 | Apache-2.0 |
| Schémas XSD Factur-X (FNFE-MPE / FeRD) : BASIC, EN 16931 et EXTENDED de ZUGFeRD 2.5.2, MINIMUM et BASIC WL de Factur-X 1.09 | 1.09.2 | Apache-2.0 |
| Schémas XSD UN/CEFACT CII D16B (`resources/cii/16b/xsd` de la configuration du validateur KoSIT XRechnung 3.0.2, release 2026-08-31) | D16B | Copyright UN/CEFACT, copie et diffusion autorisées (avis en tête des fichiers) ; configuration KoSIT sous Apache-2.0 |
| Schémas XSD UN/CEFACT CII D22B (`CII/1xsd-CII_D22B_uncoupled` de fnfempe/France_RFE), compilés pour les tests | D22B | Schémas UN/CEFACT, sans avis dans les fichiers ; dépôt France_RFE sous Apache-2.0 |
| Schémas XSD UBL 2.1 (OASIS, os-UBL-2.1, schémas `xsdrt` d'Invoice et CreditNote) | 2.1 | Copyright OASIS Open, politique de droits de propriété intellectuelle d'OASIS |
| Schematrons Factur-X MINIMUM et BASIC WL (FNFE-MPE / FeRD) | 1.09 | Apache-2.0 |
| Schematrons Factur-X BASIC, EN 16931 et EXTENDED et leurs listes de codes (FeRD, ZUGFeRD 2.5.2), assertions de listes de codes | 1.09.2 | Apache-2.0 |
| XSLT de validation EXTENDED (FeRD, ZUGFeRD 2.5.2) | 1.09.2 | Apache-2.0 |
| Schematrons Peppol BIS Billing 3.0, CII et UBL (OpenPEPPOL/peppol-bis-invoice-3) | 3.0.21 | aucune licence publiée dans le dépôt |
| Schematrons NLCIUS-CII et SI-UBL 2.0 (autorité Peppol néerlandaise, peppolautoriteit-nl/validation) | 1.0.3.13 et 2.0.3.13 (release 2026-05-21) | MIT (dépôt) ; EUPL-1.2 selon l'en-tête du schematron CII |

Les fichiers de `testdata/` à la racine viennent du corpus ZUGFeRD (Apache-2.0) et, pour l'avoir UBL, des artefacts CEN (EUPL-1.2), voir [testdata/NOTICE](testdata/NOTICE). Les factures de test des règles françaises, allemandes, Peppol et néerlandaises ont été écrites à la main, avec des parties fictives (la facture Peppol UBL est la facture Peppol CII réécrite par `ubl.Marshal`).
