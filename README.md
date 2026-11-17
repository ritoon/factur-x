# factur-x

<p align="center">
  <img src="docs/gopher.webp" alt="Le gopher Go en chapeau de papier et nœud papillon, tapant des factures à la machine à écrire entre des piles de factures" width="360">
</p>

Package Go pour les factures électroniques hybrides **Factur-X**, **ZUGFeRD** et **XRechnung**. Il prend un PDF déjà produit par votre application et y ajoute ce que la norme exige ; il relit et valide n'importe quelle facture de ces formats, quel que soit son profil. Il lit, écrit et valide aussi les factures XML en syntaxe **UBL 2.1**.

- Pur Go, bibliothèque standard uniquement : pas de CGO, pas de JVM, pas de binaire externe, pas d'appel réseau.
- Factur-X 1.0x = ZUGFeRD 2.x, syntaxe CII, profils MINIMUM, BASIC WL, BASIC, EN 16931, EXTENDED, XRECHNUNG et EXTENDED-CTC-FR ; schémas et règles de profil à jour de Factur-X 1.09.2 (ZUGFeRD 2.5.2). Factures Peppol BIS Billing 3.0 en CII.
- UBL 2.1 (Invoice et CreditNote, liaison EN 16931 de la CEN/TS 16931-3-2) : lecture et écriture dans le même modèle que le CII, validation contre le schéma XSD UBL 2.1 et selon les artefacts EN 16931 pour UBL ; la syntaxe d'un XML est détectée d'après son élément racine.
- Validation complète sans Java : schéma XSD (du profil Factur-X, ou UBL 2.1), règles EN 16931 (CII et UBL), règles de profil Factur-X, règles françaises BR-FR (réforme 2026) et allemandes BR-DE (XRechnung), règles Peppol BIS Billing 3.0 (CII), contrôle du conteneur PDF.
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

pdf, err := facturx.Create(host, inv, &facturx.CreateOptions{
	Calculate: true, // montants de ligne, totaux et ventilation de TVA
	Validate:  &validate.Options{Rules: validate.Default | validate.France},
})
```

- `Calculate` remplit les montants que la norme déduit des autres (BT-131, BT-106 à BT-115, ventilation BG-23), en décimaux exacts.
- Avec `Validate`, `Create` refuse d'injecter un XML qui a des erreurs et renvoie un `*facturx.ValidationError` qui porte le rapport complet.
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

Pour injecter un XML produit ailleurs : `facturx.Embed(pdf, xml, nil)`. Pour écrire le XML seul : `cii.Marshal(inv, &cii.MarshalOptions{Profile: invoice.ProfileBasic})` ; seuls les éléments que le profil autorise sont écrits, dans l'ordre de son schéma, et l'option `Strict` signale les données perdues.

#### Pièces jointes complémentaires

Factur-X admet d'autres pièces jointes que la facture : bon de commande, relevé de temps, bon de livraison… `EmbedOptions.Attachments` (et donc `CreateOptions.Embed`) les écrit comme le XML : flux avec type MIME et date de modification, `/F` et `/UF`, `/AFRelationship /Supplement` par défaut, entrée dans `/EmbeddedFiles` et dans le `/AF` du catalogue.

```go
pdf, err := facturx.Embed(host, xml, &facturx.EmbedOptions{
	Attachments: []facturx.File{
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
inv, att, err := facturx.Read(pdf)
// inv : modèle EN 16931 (inv.Number, inv.Seller, inv.Lines, inv.Totals…)
// att : nom du fichier, AFRelationship, présence dans /AF, métadonnées XMP
```

`facturx.Extract` renvoie seulement le XML. `cii.Parse` lit un XML CII seul, `ubl.Parse` une facture ou un avoir UBL 2.1 (voir [UBL](#ubl)). La lecture est tolérante : ZUGFeRD 2.0 et 2.1, préfixes quelconques, identifiants de profil mal orthographiés trouvés dans la nature ; une valeur illisible est signalée sans arrêter la lecture du reste.

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

### Valider

```go
report, att, err := facturx.Check(pdf, &validate.Options{Auto: true})
for _, f := range report.Findings {
	fmt.Println(f.Severity, f.Rule, f.Message, f.Path, f.Line)
}
if !report.Valid() { /* au moins une erreur */ }
```

`Check` contrôle le conteneur PDF puis le XML. Pour un XML seul : `validate.XML(xml, opts)`. Le rapport ne s'arrête jamais à la première erreur ; chaque constat porte l'identifiant officiel de la règle, sa gravité, sa source, le chemin XPath et la ligne.

`validate.XML` détecte la syntaxe. Un `Invoice` ou un `CreditNote` UBL 2.1 est vérifié contre le schéma XSD OASIS UBL 2.1 de son document, avec les mêmes règles XSD-* que le CII, puis contrôlé selon les artefacts EN 16931 pour UBL : règles métier et listes de codes avec leurs contextes UBL, règles de syntaxe UBL-CR (éléments et attributs qu'EN 16931 n'utilise pas, en avertissement), UBL-SR (cardinalités) et UBL-DT (types de données). Les règles de profil Factur-X, propres au CII, ne s'appliquent pas. Les règles BR-FR, BR-DE et Chorus Pro s'appliquent à une vue CII du document ; leurs constats sont situés dans l'UBL. Les règles Peppol, écrites pour le CII, ne s'appliquent pas (voir [Limites](#limites)). Un PDF Factur-X qui embarque un UBL est refusé : Factur-X impose le CII.

| Option | Effet |
|---|---|
| `nil` ou `&validate.Options{}` | Schéma (du profil, ou UBL 2.1), règles EN 16931, règles de profil Factur-X ; règles BR-DE en plus pour une facture XRECHNUNG, règles Peppol pour une facture CII Peppol BIS Billing 3.0 |
| `Auto: true` | Ajoute les règles du pays du vendeur (BT-40) : BR-FR et profils admis en France pour un vendeur en France, profils acceptés par le BMF pour un vendeur en Allemagne ; `Check` y ajoute l'`/AFRelationship` admise dans ce pays (voir plus bas) |
| `Rules: validate.Default \| validate.France` | Choix explicite des jeux de règles (`France`, `Germany`, `GermanyProfile`, `ChorusPro`, `Peppol`) ; les règles BR-DE d'une facture XRECHNUNG et les règles Peppol d'une facture Peppol s'y ajoutent toujours |
| `RejectBasicWL: true` | Avec les règles françaises, signale BASIC WL en erreur (FR-PROFILE-03) |
| `Rules: … \| validate.ChorusPro` | Ajoute les contrôles B2G de Chorus Pro (voir ci-dessous) |
| `Rules: … \| validate.Peppol` | Ajoute les règles Peppol BIS Billing 3.0 à une facture qui ne se déclare pas Peppol (voir ci-dessous) |
| `Profile: invoice.ProfileBasic` | Valide contre un profil plutôt que celui que déclare BT-24 |

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

Le jeu `Peppol` applique à une facture CII les 90 assertions du schematron Peppol BIS Billing 3.0 : règles PEPPOL-EN16931-R (processus métier BT-23, identifiant de spécification BT-24, référence acheteur ou de commande, adresses électroniques du vendeur et de l'acheteur, une seule note, totaux de TVA, remises et charges, montant net des lignes, périodes de ligne), formats des identifiants selon leur schéma ICD (PEPPOL-COMMON-R : GLN, numéros d'organisation norvégien, suédois, danois, belge, néerlandais, codes italiens, ABN australien…), listes de codes restreintes, et règles nationales norvégiennes, danoises, italiennes, suédoises et néerlandaises, appliquées selon le pays du vendeur.

Il s'applique d'office quand BT-24 est l'identifiant Peppol (`urn:cen.eu:en16931:2017#compliant#urn:fdc:peppol.eu:2017:poacc:billing:3.0`, profil `invoice.ProfilePeppol`), quels que soient les jeux choisis dans `Rules`, sinon seulement sur demande (`Rules: validate.Default | validate.Peppol`, `facturx validate -peppol`). `Auto` ne l'ajoute jamais : la plupart des factures Factur-X et ZUGFeRD valides ne respectent pas ces règles (pas de BT-23 Peppol, pas d'adresse électronique, pas de référence acheteur), et les appliquer sans condition les signalerait à tort. Il n'est pas appliqué à une facture UBL, dont le schematron Peppol est différent : le rapport porte alors une information (PEPPOL-UBL).

Les règles reproduisent le schematron tel qu'il est écrit, y compris là où il vise l'UBL plus que le CII :

- les identifiants de TVA sont cherchés avec le `schemeID` `VAT` de l'UBL : un identifiant CII (`VA`) ne sert ni à établir le pays du vendeur (c'est alors son adresse qui décide des règles norvégiennes et italiennes), ni aux contrôles NO-R-001, SE-R-001 et SE-R-002 ;
- PEPPOL-EN16931-P0100 porte sur `ram:ExchangedDocument/ram:TypeCode`, absent d'une facture CII (le document y est `rsm:ExchangedDocument`) : la règle ne se déclenche pas ;
- DK-R-007 cherche le mandat de prélèvement sous le moyen de paiement, alors qu'il figure dans les conditions de paiement en CII.

### En ligne de commande

```bash
go install github.com/ritoon/factur-x/cmd/facturx@latest
```

```text
facturx extract [-o out.xml] facture.pdf
facturx info facture.pdf
facturx validate [-fr] [-de] [-peppol] [-auto] [-reject-basicwl] [-choruspro [-service-required] [-engagement-required]] [-profile p] [-json] facture.pdf|facture.xml
facturx embed -xml facture.xml [-profile p] [-attach fichier]... [-o out.pdf] entree.pdf
facturx json facture.pdf|facture.xml
facturx rules [-fr] [-de] [-choruspro] [-peppol]
```

`validate` et `json` acceptent un XML CII ou UBL. `validate` sort avec le code 1 quand la facture a des erreurs, ce qui permet de l'utiliser en intégration continue. Ses options de règles (`-fr`, `-de`, `-choruspro`, `-peppol`) s'ajoutent aux règles par défaut : elles ne retirent jamais les règles BR-DE d'une facture XRECHNUNG ni les règles Peppol d'une facture Peppol. `embed -attach` ajoute une pièce jointe complémentaire (`/AFRelationship /Supplement`, type MIME d'après l'extension) ; l'option se répète.

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
| PEPPOL (CII) | oui | oui | schéma EN 16931 | oui ; règles Peppol appliquées d'office |
| UBL 2.1 (EN 16931, XRECHNUNG, PEPPOL) | oui | oui | schéma UBL 2.1 (Invoice, CreditNote), sauf le contenu des extensions | sans objet ; BR-DE appliquées d'office pour XRECHNUNG ; règles Peppol non appliquées |

### Règles

| Jeu | Règles | Référence |
|---|---|---|
| Schéma | 11 contrôles (éléments, ordre, cardinalités, attributs, espaces de noms, types, listes fermées) | XSD Factur-X 1.09.2 (ZUGFeRD 2.5.2) et XSD OASIS UBL 2.1 (Invoice, CreditNote), compilés en tables Go |
| EN 16931 | 793 assertions (328 erreurs, 465 avertissements) : BR, BR-CO, BR-CL, BR-DEC, règles par catégorie de TVA (BR-S, BR-Z, BR-E, BR-AE, BR-IC, BR-IG, BR-IP, BR-G, BR-O, BR-AF, BR-AG, BR-B), CII-SR, CII-DT | Artefacts CEN TC 434 v1.3.17 (CII) |
| EN 16931 (UBL) | Mêmes règles métier, avec les contextes et les tests des artefacts UBL (catégories L et M, tolérances, BR-DEC sur le texte, listes de codes propres à l'UBL) ; 759 règles de syntaxe (62 erreurs, 697 avertissements) : UBL-CR (678), UBL-SR (57), UBL-DT (24) | Artefacts CEN TC 434 v1.3.17 (UBL) |
| Factur-X | 229 règles (207 erreurs, 22 avertissements) : identification du profil (BT-24), assertions des schematrons MINIMUM et BASIC WL, éléments non utilisés par profil ; règles BR-FXEXT d'EXTENDED et EXTENDED-CTC-FR (sous-types et regroupements de lignes, totaux avec tolérance, frais logistiques et frais pour compte de tiers, ventilation de TVA par catégorie, taux et motif d'exonération) | Schematrons Factur-X 1.09 (MINIMUM, BASIC WL) ; XSLT EXTENDED Factur-X 1.09.2 (ZUGFeRD 2.5.2) |
| France (BR-FR) | 59 règles, 170 assertions : SIREN/SIRET, adresses électroniques, mentions obligatoires, cadres de facturation, multi-vendeurs ; 3 contrôles des profils admis (FR-PROFILE) | Schematron FNFE-MPE v1.4.0.04 |
| Allemagne (BR-DE) | 52 assertions (BR-DE, BR-DE-CVD, BR-DEX, BR-TMP) ; contrôle des profils acceptés par le BMF (DE-PROFILE-01) | Schematron KoSIT XRechnung 3.0.2 ; BMF |
| Peppol BIS Billing 3.0 | 90 assertions (73 erreurs, 17 avertissements) : PEPPOL-EN16931-R, PEPPOL-COMMON-R (identifiants par schéma ICD), listes de codes restreintes, règles nationales NO, DK, IT, SE et NL | Schematron Peppol BIS Billing 3.0.21 (`PEPPOL-EN16931-CII.sch`, inchangé en 3.0.22) |
| Chorus Pro (B2G) | 8 contrôles (CPRO-01 à CPRO-08) : SIRET de la structure publique, code service, numéro d'engagement, type de document, numéro de facture | Spécifications externes Chorus Pro (annexe EDI V4.23) |
| Conteneur PDF | 40 contrôles (PDF-01 à PDF-40) : PDF/A-3 ou PDF/A-4f déclaré, cohérent avec la version PDF, XMP `fx` cohérent avec le XML, pièce jointe, `/AF`, `/AFRelationship` selon le profil et le pays, type MIME, chiffrement ; prérequis PDF/A vérifiables sans rendu : polices incorporées, JavaScript et actions interdites (`/OpenAction`, liens, signets, `/AA`), cohérence entre `/Info` et le XMP, pièces jointes annexes (`/AFRelationship`, type MIME, présence dans un `/AF`), filtre `LZWDecode` | Spécification Factur-X 1.08 et 1.09 (§ 6.2.2), ISO 19005-3 et 19005-4 |

Les messages sont ceux des artefacts officiels. Les listes de codes (pays, devises, unités, schémas d'identifiants, motifs d'exonération…) sont embarquées.

### Exactitude mesurée

Sur les 26 XML CII valides du corpus officiel `XML-Rechnung/CII`, le validateur signale exactement les 6 fichiers que signalent les schematrons officiels (exécutés par papierleicht) : BR-CL-25 deux fois, CII-SR-465 quatre fois sur les exemples XRECHNUNG. Les 104 PDF hybrides du corpus (`ZUGFeRDv2/correct`, `XML-Rechnung/FX`) sont tous extraits et leurs 26 XML tous lus.

Les 73 factures UBL du corpus (`XML-Rechnung/UBL`, 28) et de la suite de tests KoSIT (45) sont lues, réécrites puis relues dans le même modèle, et validées sans autre erreur que les 3 attendues : BR-CL-25 sur les deux exemples au schéma 9958, comme en CII, et l'exemple `not_validating…`, invalide à dessein. Toutes respectent le schéma UBL 2.1. Sur 11 000 variantes de ces factures modifiées au hasard (ordre, cardinalités, types, attributs, espaces de noms, extensions), le contrôle du schéma rend le même verdict que xmllint (libxml2) avec les XSD OASIS.

Les règles Peppol donnent le résultat attendu sur les 351 tests unitaires CII du dépôt Peppol BIS Billing 3.0 (87 jeux de tests, dossiers `rules/unit-CII-*`, versions 3.0.21 et 3.0.22) ; les trois assertions qu'ils ne couvrent pas (PEPPOL-COMMON-R040, R041, R056-2) le sont par les tests du package.

Sur les 147 XML CII lus du corpus ZUGFeRD et des XML extraits de ses PDF, 145 sont réécrits en EXTENDED avec tout leur arbre ; les 2 autres portent des données qu'aucun profil Factur-X n'admet (le code banque allemand de ZUGFeRD 2.0, retiré depuis du schéma, et un type de référence sur un document justificatif autre que 130), et `Marshal` les signale. Un document qui contient chaque élément du XSD EXTENDED avec tous ses attributs est relu et réécrit à l'identique, sauf ce même élément interdit par les règles.

### PDF

Le lecteur PDF est écrit pour ce package : tables xref classiques et flux xref, flux d'objets, mises à jour incrémentales, reconstruction quand la table xref est endommagée, filtres Flate (avec prédicteurs), ASCIIHex et ASCII85. La décompression est bornée (256 Mio par flux) et la profondeur d'imbrication limitée, pour qu'un fichier hostile ne puisse ni saturer la mémoire ni faire déborder la pile. Le décompresseur est testé par fuzzing différentiel contre `compress/flate`.

Le lecteur XML est lui aussi écrit pour ce package : un seul passage sur les octets, espaces de noms résolus, imbrication limitée à 256 niveaux, déclarations DOCTYPE ignorées et aucune entité développée hors des cinq prédéfinies. Il est testé par fuzzing différentiel contre `encoding/xml` : mêmes documents acceptés, même arbre. La lecture rapide de BT-24 (le profil de la pièce jointe, lu par `Embed` et `CheckContainer` sans construire d'arbre) est à son tour comparée à ce lecteur par fuzzing : même élément racine, même identifiant.

## Limites

- **Pas de conversion PDF/A.** `Embed` déclare PDF/A-3 (ou PDF/A-4f pour un PDF 2.0) dans le XMP mais ne touche ni aux polices, ni aux espaces de couleur, ni aux output intents : le PDF hôte doit déjà respecter PDF/A-2, PDF/A-3 ou PDF/A-4 (polices incorporées, output intent), ce que la plupart des générateurs savent faire sur option. `Check` signale ce qui manque parmi ce qui se vérifie sans rendu (output intent, polices non incorporées, JavaScript et actions interdites, `/Info` incohérent avec le XMP, filtre LZW) ; il ne remplace pas un validateur PDF/A comme veraPDF : les espaces de couleur, la transparence, les programmes de police eux-mêmes et le contenu des pages ne sont pas analysés. Une police présente dans les ressources d'une page sans être incorporée est un avertissement (PDF-31), le contenu n'étant pas lu pour savoir si elle sert à afficher du texte visible.
- **UBL : extensions non vérifiées, pas de règles Peppol, règles nationales en identifiants CII.** Le schéma UBL 2.1 déclare le contenu des extensions (`ext:ExtensionContent`) en `xsd:any` lax, et les schémas de signature qu'il importe pour cela (xmldsig, XAdES) ne sont pas compilés : seuls l'espace de noms et le nombre des éléments d'une extension sont vérifiés, pas leur contenu. Les règles BR-FR, BR-DE et Chorus Pro tournent sur une vue CII du document : leurs constats sont situés dans l'UBL, mais portent les identifiants et les messages des schematrons CII. Les règles propres aux schematrons UBL de ces jeux ne sont pas encore écrites (dates BR-FR-03 sur BT-134 et BT-135, identifiants scindés comme BR-FR-01_BT-1-1 ; BR-DE-1, BR-DE-23-a à BR-DE-25-a, BR-TMP-2 et PEPPOL-EN16931-R043 de XRechnung). Les règles Peppol ne s'appliquent qu'aux factures CII, alors que les factures Peppol circulent le plus souvent en UBL (c'est la syntaxe des exemples officiels et du réseau) : le schematron UBL de Peppol a d'autres contextes, d'autres tests et des règles propres, et les règles CII appliquées à une vue CII s'écartent du résultat attendu sur 179 des 683 assertions des tests unitaires UBL officiels. Pour une facture UBL, le jeu Peppol ne donne qu'une information (PEPPOL-UBL).
- **PDF chiffrés** : signalés (PDF-01), pas déchiffrés.
- **ZUGFeRD 1.0** : le XMP est reconnu, le XML (syntaxe CII antérieure à D16B) n'est pas lu.
- Les règles des schematrons sont réécrites en Go, pas interprétées : une nouvelle version des artefacts officiels demande une mise à jour du code (les catalogues EN 16931 du CII et de l'UBL sont régénérés par `go generate`, les tests vérifient l'alignement).

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
| `ubl` | Lecture et écriture du XML UBL 2.1 (Invoice, CreditNote) |
| `validate` | Validation : schéma, règles EN 16931 (CII et UBL), règles de profil Factur-X, règles BR-FR, BR-DE et Peppol |
| `cmd/facturx` | Outil en ligne de commande |
| `internal/…` | Lecteur et écrivain PDF, XMP, arbre XML, règles, listes de codes, tables de schéma |
| `bench/ecosystem` | Benchmarks comparatifs avec les autres librairies Go (modules séparés) |

## Tests

```bash
go test ./...
```

Les tests de base n'ont besoin que du dépôt ; la CI ([.github/workflows/ci.yml](.github/workflows/ci.yml)) les exécute avec `-race` sur Go 1.24 et sur la dernière version, après `gofmt` et `go vet`, et fait tourner 30 secondes de fuzzing sur le lecteur PDF, le codec Flate, le lecteur XML et la lecture de BT-24 dans la pièce jointe. Des tests supplémentaires s'activent quand les artefacts officiels sont disponibles en local :

| Variable | Contenu | Tests |
|---|---|---|
| `FX_CORPUS` | Clone de [ZUGFeRD/corpus](https://github.com/ZUGFeRD/corpus) | Extraction et contrôle des PDF du corpus ; lecture, aller-retour, schéma UBL 2.1 et validation des factures UBL de `XML-Rechnung/UBL` |
| `FX_CORPUS_XML` | XML extraits du corpus | Lecture, validation et aller-retour lecture → écriture : sans perte signalée, l'arbre complet du document d'origine doit se retrouver dans le XML réécrit, dans son profil et en EXTENDED |
| `FX_XSD` | XSD Factur-X 1.09.2, un dossier par profil (`MINIMUM`, `BASICWL`, `BASIC`, `EN16931`, `EXTENDED`) | Le XML écrit par `cii.Marshal` est vérifié contre les XSD d'origine |
| `FX_XSD_DIR` | XSD Factur-X 1.09.2 : paquet FeRD ZUGFeRD 2.5.2 (BASIC, EN 16931, EXTENDED) et paquet FNFE-MPE 1.09 (MINIMUM, BASIC WL, que le corrigendum 1.09.2 ne modifie pas) | Les tables de schéma régénérées sont identiques à celles du dépôt |
| `FX_KOSIT` | Clone de [itplr-kosit/xrechnung-testsuite](https://github.com/itplr-kosit/xrechnung-testsuite) | Toutes les factures CII de la suite de tests KoSIT passent les règles BR-DE ; ses factures UBL sont lues, réécrites, vérifiées contre le schéma UBL 2.1 et validées sans erreur |
| `FX_PEPPOL` | Clone de [OpenPEPPOL/peppol-bis-invoice-3](https://github.com/OpenPEPPOL/peppol-bis-invoice-3), version 3.0.21 ou suivante | Les 351 tests unitaires CII (`rules/unit-CII-*`) donnent le résultat attendu par le schematron |
| `FX_UBL_ZIP` | Paquet OASIS [UBL-2.1.zip](https://docs.oasis-open.org/ubl/os-UBL-2.1/UBL-2.1.zip) | Les tables du schéma UBL régénérées sont identiques à celles du dépôt (`go generate -run -ubl` dans `internal/schema` télécharge le paquet et vérifie son SHA-256) |

## Licence et provenance

Le code est sous licence MIT (voir [LICENSE](LICENSE)).

Les règles sont réécrites en Go à partir des artefacts officiels, qui ne sont pas inclus dans le dépôt (les schémas XSD sont compilés en tables) ; leurs identifiants et leurs messages sont repris pour que les rapports correspondent aux validateurs de référence :

| Artefact | Version | Licence |
|---|---|---|
| Artefacts de validation EN 16931 pour CII et UBL (CEN TC 434, ConnectingEurope/eInvoicing-EN16931) | 1.3.17 | EUPL-1.2 |
| Schematron BR-FR (FNFE-MPE, fnfempe/France_RFE) | 1.4.0.04 | EUPL-1.2 |
| Schematron XRechnung (KoSIT) | 3.0.2 | Apache-2.0 |
| Schémas XSD Factur-X (FNFE-MPE / FeRD) : BASIC, EN 16931 et EXTENDED de ZUGFeRD 2.5.2, MINIMUM et BASIC WL de Factur-X 1.09 | 1.09.2 | Apache-2.0 |
| Schémas XSD UBL 2.1 (OASIS, os-UBL-2.1, schémas `xsdrt` d'Invoice et CreditNote) | 2.1 | Copyright OASIS Open, politique de droits de propriété intellectuelle d'OASIS |
| Schematrons Factur-X MINIMUM et BASIC WL (FNFE-MPE / FeRD) | 1.09 | Apache-2.0 |
| XSLT de validation EXTENDED (FeRD, ZUGFeRD 2.5.2) | 1.09.2 | Apache-2.0 |
| Schematron Peppol BIS Billing 3.0, CII (OpenPEPPOL/peppol-bis-invoice-3) | 3.0.21 | aucune licence publiée dans le dépôt |

Les fichiers de `testdata/` à la racine viennent du corpus ZUGFeRD (Apache-2.0) et, pour l'avoir UBL, des artefacts CEN (EUPL-1.2), voir [testdata/NOTICE](testdata/NOTICE). Les factures de test des règles françaises, allemandes et Peppol ont été écrites à la main, avec des parties fictives.
