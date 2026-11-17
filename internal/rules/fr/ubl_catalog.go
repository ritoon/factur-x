package fr

import "github.com/ritoon/zerechnung/internal/rules"

// UBLCatalog lists the assertions that Check applies to a UBL invoice, in
// the order of the schematron BR-FR-Flux2-Schematron-UBL version 1.4.0.04.
// Every assertion is fatal (severity Error). An identifier shared with
// Catalog has the same description. BR-FR-16_BT-103 and BR-FR-DEC-01_BT-111
// are left out: their rules follow a rule of the same context in the same
// pattern, so the reference never reports them (see the package
// documentation).
var UBLCatalog = []rules.RuleInfo{
	// BR-FR-01.
	e("BR-FR-01_BT-1-1", "L'identifiant de facture (BT-1) ne dépasse pas 35 caractères."),
	e("BR-FR-01_BT-1-2", "L'identifiant de facture (BT-1) ne contient que des caractères alphanumériques et + - _ /, sans espace."),
	e("BR-FR-01_BT-25-1", "L'identifiant de facture référencée (BT-25) ne dépasse pas 35 caractères."),
	e("BR-FR-01_BT-25-2", "L'identifiant de facture référencée (BT-25) ne contient que des caractères alphanumériques et + - _ /, sans espace."),
	e("BR-FR-01_EXT-FR-FE-136-1", "L'identifiant de facture référencée en ligne (EXT-FR-FE-136) ne dépasse pas 35 caractères."),
	e("BR-FR-01_EXT-FR-FE-136-2", "L'identifiant de facture référencée en ligne (EXT-FR-FE-136) ne contient que des caractères alphanumériques et + - _ /, sans espace."),
	// BR-FR-02.
	e("BR-FR-02_BT-1", "L'identifiant de facture (BT-1) ne contient que des caractères alphanumériques et + - _ /, sans espace."),
	e("BR-FR-02_BT-25", "L'identifiant de facture référencée (BT-25) ne contient que des caractères alphanumériques et + - _ /, sans espace."),
	e("BR-FR-02_EXT-FR-FE-136", "L'identifiant de facture référencée en ligne (EXT-FR-FE-136) ne contient que des caractères alphanumériques et + - _ /, sans espace."),
	// BR-FR-03.
	e("BR-FR-03_BT-2", "La date d'émission (BT-2) est une date valide des années 2000 à 2099 (AAAAMMJJ en CII, AAAA-MM-JJ en UBL)."),
	e("BR-FR-03_BT-7", "La date de fait générateur de la TVA (BT-7) est une date valide des années 2000 à 2099 (AAAAMMJJ en CII, AAAA-MM-JJ en UBL)."),
	e("BR-FR-03_BT-9", "La date d'échéance (BT-9) est une date valide des années 2000 à 2099 (AAAAMMJJ en CII, AAAA-MM-JJ en UBL)."),
	e("BR-FR-03_BT-26", "La date de la facture référencée (BT-26) est une date valide des années 2000 à 2099 (AAAAMMJJ en CII, AAAA-MM-JJ en UBL)."),
	e("BR-FR-03_BT-72", "La date de livraison effective (BT-72) est une date valide des années 2000 à 2099 (AAAAMMJJ en CII, AAAA-MM-JJ en UBL)."),
	e("BR-FR-03_BT-73", "La date de début de période de facturation (BT-73) est une date valide des années 2000 à 2099 (AAAAMMJJ en CII, AAAA-MM-JJ en UBL)."),
	e("BR-FR-03_BT-74", "La date de fin de période de facturation (BT-74) est une date valide des années 2000 à 2099 (AAAAMMJJ en CII, AAAA-MM-JJ en UBL)."),
	e("BR-FR-03_EXT-FR-FE-138", "La date de la facture référencée en ligne (EXT-FR-FE-138) est une date valide des années 2000 à 2099 (AAAAMMJJ en CII, AAAA-MM-JJ en UBL)."),
	e("BR-FR-03_EXT-FR-FE-158", "La date de livraison effective en ligne (EXT-FR-FE-158) est une date valide des années 2000 à 2099 (AAAAMMJJ en CII, AAAA-MM-JJ en UBL)."),
	e("BR-FR-03_BT-134", "La date de début de période de la ligne (BT-134) est une date AAAA-MM-JJ valide des années 2000 à 2099."),
	e("BR-FR-03_BT-135", "La date de fin de période de la ligne (BT-135) est une date AAAA-MM-JJ valide des années 2000 à 2099."),
	// BR-FR-04.
	e("BR-FR-04_BT-3", "Le code type de document (BT-3) appartient à la liste française."),
	e("BR-FR-04_EXT-FR-FE-02", "Le code type du document référencé (EXT-FR-FE-02) appartient à la liste française."),
	e("BR-FR-04_EXT-FR-FE-137", "Le code type du document référencé en ligne (EXT-FR-FE-137) appartient à la liste française."),
	// BR-FR-05.
	e("BR-FR-05_BT-22-1", "Une note #PMT# (indemnité forfaitaire pour frais de recouvrement) est présente."),
	e("BR-FR-05_BT-22-2", "Une note #PMD# (pénalités de retard) est présente."),
	e("BR-FR-05_BT-22-3", "Une note #AAB# (escompte ou absence d'escompte) est présente."),
	// BR-FR-06.
	e("BR-FR-06_BT-21-1", "Le code sujet PMT apparaît au plus une fois dans les notes."),
	e("BR-FR-06_BT-21-2", "Le code sujet PMD apparaît au plus une fois dans les notes."),
	e("BR-FR-06_BT-21-3", "Le code sujet AAB apparaît au plus une fois dans les notes."),
	e("BR-FR-06_BT-21-4", "Le code sujet TXD apparaît au plus une fois dans les notes."),
	// BR-FR-08.
	e("BR-FR-08_BT-23", "Le cadre de facturation (BT-23) est présent et appartient à la liste B1 … M9."),
	// BR-FR-09.
	e("BR-FR-09_BT-29", "Le SIRET du vendeur (BT-29, schéma 0009) a 14 chiffres et commence par son SIREN."),
	e("BR-FR-09_BT-46", "Le SIRET de l'acheteur (BT-46, schéma 0009) a 14 chiffres et commence par son SIREN."),
	e("BR-FR-09_BT-60", "Le SIRET du bénéficiaire (BT-60, schéma 0009) a 14 chiffres et commence par son SIREN."),
	e("BR-FR-09_EXT-FR-FE-06", "Le SIRET de l'agent de l'acheteur (EXT-FR-FE-06) a 14 chiffres et commence par son SIREN."),
	e("BR-FR-09_EXT-FR-FE-46", "Le SIRET du payeur (EXT-FR-FE-46) a 14 chiffres et commence par son SIREN."),
	e("BR-FR-09_EXT-FR-FE-69", "Le SIRET de l'agent du vendeur (EXT-FR-FE-69) a 14 chiffres et commence par son SIREN."),
	e("BR-FR-09_EXT-FR-FE-92", "Le SIRET du facturé à (EXT-FR-FE-92) a 14 chiffres et commence par son SIREN."),
	e("BR-FR-09_EXT-FR-FE-115", "Le SIRET du facturant (EXT-FR-FE-115) a 14 chiffres et commence par son SIREN."),
	e("BR-FR-09_BT-71", "Le SIRET du livré à (BT-71) a 14 chiffres et commence par son SIREN."),
	e("BR-FR-09_EXT-FR-FE-146", "Le SIRET du livré à de la ligne (EXT-FR-FE-146) a 14 chiffres et commence par son SIREN."),
	// BR-FR-10.
	e("BR-FR-10_BT-30", "Le SIREN du vendeur (BT-30, schéma 0002) est présent et composé de 9 chiffres."),
	// BR-FR-11.
	e("BR-FR-11_BT-47", "Avec le traitement BAR B2B, le SIREN de l'acheteur (BT-47, schéma 0002) est composé de 9 chiffres."),
	// BR-FR-12.
	e("BR-FR-12_BT-49", "L'adresse électronique de l'acheteur (BT-49) est présente."),
	// BR-FR-13.
	e("BR-FR-13_BT-34", "L'adresse électronique du vendeur (BT-34) est présente."),
	// BR-FR-15.
	e("BR-FR-15_BT-95_BT-102-1", "La catégorie de TVA des remises et charges (BT-95, BT-102) est S, E, AE, K, G, O ou Z."),
	e("BR-FR-15_BT-95_BT-102-2", "La catégorie de TVA des remises et charges (BT-95, BT-102) n'est pas L ou M."),
	e("BR-FR-15_BT-118-1", "La catégorie de TVA de la ventilation (BT-118) est S, E, AE, K, G, O ou Z."),
	e("BR-FR-15_BT-118-2", "La catégorie de TVA de la ventilation (BT-118) n'est pas L ou M."),
	e("BR-FR-15_BT-151-1", "La catégorie de TVA d'un article (BT-151) est S, E, AE, K, G, O ou Z."),
	e("BR-FR-08_BT-151-2", "La catégorie de TVA d'un article (BT-151) n'est pas L ou M (assertion du pattern BR-FR-15, identifiée BR-FR-08 dans le schématron UBL)."),
	// BR-FR-16.
	e("BR-FR-16_BT-96", "Le taux de TVA des remises et charges (BT-96, BT-103) est un taux français autorisé."),
	e("BR-FR-16_BT-119", "Le taux de TVA de la ventilation (BT-119) est un taux français autorisé."),
	e("BR-FR-16_BT-152", "Le taux de TVA de la ligne (BT-152) est un taux français autorisé."),
	// BR-FR-17.
	e("BR-FR-17_BT-123", "La description d'une pièce jointe (BT-123 ; document de type 916 en CII) est un code de pièce jointe autorisé."),
	// BR-FR-18.
	e("BR-FR-18_BT-123", "Au plus un document additionnel a la description LISIBLE."),
	// BR-FR-20.
	e("BR-FR-20_BT-21", "Une note de code sujet BAR contient B2B, B2BINT, B2C, B2CINT, OUTOFSCOPE ou ARCHIVEONLY."),
	// BR-FR-21.
	e("BR-FR-21_BT-49", "Avec le traitement BAR B2B hors autofacturation, l'adresse électronique de l'acheteur (BT-49) commence par son SIREN et a le schéma 0225."),
	// BR-FR-22.
	e("BR-FR-22_BT-34", "Avec le traitement BAR B2B en autofacturation, l'adresse électronique du vendeur (BT-34) commence par son SIREN et a le schéma 0225."),
	// BR-FR-23.
	e("BR-FR-23_BT-34", "L'adresse électronique 0225 du vendeur (BT-34) ne contient que [A-Za-z0-9+-_.]."),
	e("BR-FR-23_BT-49", "L'adresse électronique 0225 de l'acheteur (BT-49) ne contient que [A-Za-z0-9+-_.]."),
	e("BR-FR-23_EXT-FR-FE-12", "L'adresse électronique 0225 de l'agent de l'acheteur (EXT-FR-FE-12) ne contient que [A-Za-z0-9+-_.]."),
	e("BR-FR-23_EXT-FR-FE-29", "L'adresse électronique 0225 du bénéficiaire (EXT-FR-FE-29) ne contient que [A-Za-z0-9+-_.]."),
	e("BR-FR-23_EXT-FR-FE-52", "L'adresse électronique 0225 du payeur (EXT-FR-FE-52) ne contient que [A-Za-z0-9+-_.]."),
	e("BR-FR-23_EXT-FR-FE-75", "L'adresse électronique 0225 de l'agent du vendeur (EXT-FR-FE-75) ne contient que [A-Za-z0-9+-_.]."),
	e("BR-FR-23_EXT-FR-FE-98", "L'adresse électronique 0225 du facturé à (EXT-FR-FE-98) ne contient que [A-Za-z0-9+-_.]."),
	e("BR-FR-23_EXT-FR-FE-121", "L'adresse électronique 0225 du facturant (EXT-FR-FE-121) ne contient que [A-Za-z0-9+-_.]."),
	// BR-FR-24.
	e("BR-FR-24_BT-29", "L'identifiant privé 0224 du vendeur (BT-29) ne contient que [A-Za-z0-9+-_.]."),
	e("BR-FR-24_BT-46", "L'identifiant privé 0224 de l'acheteur (BT-46) ne contient que [A-Za-z0-9+-_.]."),
	// BR-FR-25.
	e("BR-FR-25_BT-34", "L'adresse électronique du vendeur (BT-34) ne dépasse pas 125 caractères."),
	e("BR-FR-25_BT-49", "L'adresse électronique de l'acheteur (BT-49) ne dépasse pas 125 caractères."),
	e("BR-FR-25_EXT-FR-FE-12", "L'adresse électronique de l'agent de l'acheteur (EXT-FR-FE-12) ne dépasse pas 125 caractères."),
	e("BR-FR-25_EXT-FR-FE-29", "L'adresse électronique du bénéficiaire (EXT-FR-FE-29) ne dépasse pas 125 caractères."),
	e("BR-FR-25_EXT-FR-FE-52", "L'adresse électronique du payeur (EXT-FR-FE-52) ne dépasse pas 125 caractères."),
	e("BR-FR-25_EXT-FR-FE-75", "L'adresse électronique de l'agent du vendeur (EXT-FR-FE-75) ne dépasse pas 125 caractères."),
	e("BR-FR-25_EXT-FR-FE-98", "L'adresse électronique du facturé à (EXT-FR-FE-98) ne dépasse pas 125 caractères."),
	e("BR-FR-25_EXT-FR-FE-121", "L'adresse électronique du facturant (EXT-FR-FE-121) ne dépasse pas 125 caractères."),
	// BR-FR-26.
	e("BR-FR-26_BT-29", "L'identifiant privé 0224 du vendeur (BT-29) ne dépasse pas 100 caractères."),
	e("BR-FR-26_BT-46", "L'identifiant privé 0224 de l'acheteur (BT-46) ne dépasse pas 100 caractères."),
	// BR-FR-27.
	e("BR-FR-27_BG-32", "Un attribut d'article (BG-32) a un nom (BT-160) ou un code (EXT-FR-FE-159)."),
	e("BR-FR-27_BT-160", "Le nom d'attribut d'article (BT-160) n'est pas vide."),
	e("BR-FR-27_EXT-FR-FE-159", "Le code d'attribut d'article (EXT-FR-FE-159) n'est pas vide."),
	// BR-FR-28.
	e("BR-FR-28-Value", "Un attribut d'article a une valeur (BT-161) ou une valeur mesurée avec unité, mais pas les deux."),
	// BR-FR-29.
	e("BR-FR-29_BT-18", "Les schémas AFL et AVV des objets facturés (BT-18) apparaissent au plus une fois chacun."),
	e("BR-FR-29_AFL", "L'identifiant d'objet facturé de schéma AFL (BT-18) n'est pas vide."),
	e("BR-FR-29_AVV", "L'identifiant d'objet facturé de schéma AVV (BT-18) n'est pas vide."),
	// BR-FR-30.
	e("BR-FR-30_BT-128", "Les schémas AFL et AVV des objets facturés de ligne (BT-128) apparaissent au plus une fois chacun."),
	e("BR-FR-30_AFL", "L'identifiant d'objet facturé de ligne de schéma AFL (BT-128) n'est pas vide."),
	e("BR-FR-30_AVV", "L'identifiant d'objet facturé de ligne de schéma AVV (BT-128) n'est pas vide."),
	// BR-FR-31.
	e("BR-FR-30_BT-21", "Les notes BAR portent au plus un traitement BAR (pattern BR-FR-31, identifié BR-FR-30_BT-21 dans le schématron UBL)."),
	// BR-FR-32.
	e("BR-FR-32-LEGALID", "Tout identifiant légal de schéma 0002 est composé de 9 chiffres."),
	e("BR-FR-32-ID", "Tout identifiant de partie de schéma 0002 ou 0231 est composé de 9 chiffres."),
	// BR-FR-CO-03.
	e("BR-FR-CO-03_BT-3", "Un avoir remise globale (262) comporte le numéro de contrat (BT-12) et la période de facturation (BG-14)."),
	// BR-FR-CO-04.
	e("BR-FR-CO-04_BT-3", "Une facture rectificative (384, 471, 472, 473) comporte une et une seule référence à une facture antérieure."),
	// BR-FR-CO-05.
	e("BR-FR-CO-05_BT-3", "Un avoir (261, 381, 396, 502, 503) référence une facture antérieure datée en entête ou sur chaque ligne DETAIL."),
	// BR-FR-CO-07.
	e("BR-FR-CO-07_BT-9", "La date d'échéance (BT-9) n'est pas antérieure à la date de facture (BT-2), hors acomptes et cadres B2, S2, M2."),
	// BR-FR-CO-08.
	e("BR-FR-CO-08_BT-23", "Avec les cadres B4, S4, M4, le type de facture n'est pas un acompte (386, 500, 503)."),
	// BR-FR-CO-09.
	e("BR-FR-CO-09_BT-23-1", "Avec les cadres B2, S2, M2, le montant déjà payé (BT-113) est égal au total TTC (BT-112)."),
	e("BR-FR-CO-09_BT-23-2", "Avec les cadres B2, S2, M2, le net à payer (BT-115) est nul."),
	e("BR-FR-CO-09_BT-23-3", "Avec les cadres B2, S2, M2, la date d'échéance (BT-9) est renseignée."),
	// BR-FR-CO-10.
	e("BR-FR-CO-10_BT-29-1", "Chaque identifiant du vendeur (BT-29) a un schéma."),
	e("BR-FR-CO-10_BT-29-2", "Chaque schéma d'identifiant du vendeur (BT-29) est unique."),
	e("BR-FR-CO-10_BT-46-1", "Chaque identifiant de l'acheteur (BT-46) a un schéma."),
	e("BR-FR-CO-10_BT-46-2", "Chaque schéma d'identifiant de l'acheteur (BT-46) est unique."),
	e("BR-FR-CO-10_BT-60-1", "Chaque identifiant du bénéficiaire (BT-60) a un schéma."),
	e("BR-FR-CO-10_BT-60-2", "Chaque schéma d'identifiant du bénéficiaire (BT-60) est unique."),
	e("BR-FR-CO-10_EXT-FR-FE-06-1", "Chaque identifiant de l'agent de l'acheteur (EXT-FR-FE-06) a un schéma."),
	e("BR-FR-CO-10_EXT-FR-FE-06-2", "Chaque schéma d'identifiant de l'agent de l'acheteur (EXT-FR-FE-06) est unique."),
	e("BR-FR-CO-10_EXT-FR-FE-46-1", "Chaque identifiant du payeur (EXT-FR-FE-46) a un schéma."),
	e("BR-FR-CO-10_EXT-FR-FE-46-2", "Chaque schéma d'identifiant du payeur (EXT-FR-FE-46) est unique."),
	e("BR-FR-CO-10_EXT-FR-FE-69-1", "Chaque identifiant de l'agent du vendeur (EXT-FR-FE-69) a un schéma."),
	e("BR-FR-CO-10_EXT-FR-FE-69-2", "Chaque schéma d'identifiant de l'agent du vendeur (EXT-FR-FE-69) est unique."),
	e("BR-FR-CO-10_EXT-FR-FE-92-1", "Chaque identifiant du facturé à (EXT-FR-FE-92) a un schéma."),
	e("BR-FR-CO-10_EXT-FR-FE-92-2", "Chaque schéma d'identifiant du facturé à (EXT-FR-FE-92) est unique."),
	e("BR-FR-CO-10_EXT-FR-FE-115-1", "Chaque identifiant du facturant (EXT-FR-FE-115) a un schéma."),
	e("BR-FR-CO-10_EXT-FR-FE-115-2", "Chaque schéma d'identifiant du facturant (EXT-FR-FE-115) est unique."),
	e("BR-FR-CO-10_BT-71-1", "Chaque identifiant du livré à (BT-71) a un schéma."),
	e("BR-FR-CO-10_BT-71-2", "Chaque schéma d'identifiant du livré à (BT-71) est unique."),
	e("BR-FR-CO-10_EXT-FR-FE-146-1", "Chaque identifiant du livré à de la ligne (EXT-FR-FE-146) a un schéma."),
	e("BR-FR-CO-10_EXT-FR-FE-146-2", "Chaque schéma d'identifiant du livré à de la ligne (EXT-FR-FE-146) est unique."),
	// BR-FR-CO-12.
	e("BR-FR-CO-12_BT-5", "Une facture hors EUR indique la devise de comptabilité EUR (BT-6) et le total de TVA en EUR (BT-111)."),
	// BR-FR-CO-14.
	e("BR-FR-CO-14_BT-29-1", "Un vendeur membre d'un assujetti unique (0231) porte la note TXD MEMBRE_ASSUJETTI_UNIQUE."),
	// BR-FR-CO-15.
	e("BR-FR-CO-15_BT-29-1", "Un vendeur membre d'un assujetti unique (0231) a un représentant fiscal (BG-11) avec un numéro de TVA (BT-63)."),
	// BR-FR-DEC-01.
	e("BR-FR-DEC-01_BT-93_BT-100", "Les montants de base des remises et charges (BT-93, BT-100) ont au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-92_BT-99", "Les montants de remise et de charge (BT-92, BT-99) ont au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-106", "La somme des montants nets des lignes (BT-106) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-107", "La somme des remises (BT-107) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-108", "La somme des charges (BT-108) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-109", "Le total HT (BT-109) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-110", "Les totaux de TVA (BT-110 et BT-111) ont au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-112", "Le total TTC (BT-112) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-113", "Le montant payé (BT-113) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-114", "L'arrondi (BT-114) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-115", "Le net à payer (BT-115) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-116", "La base de la ventilation de TVA (BT-116) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-117", "Le montant de la ventilation de TVA (BT-117) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-131", "Le montant net de la ligne (BT-131) a au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-136_BT-141", "Les remises et charges de ligne (BT-136, BT-141) ont au plus 19 chiffres dont 2 décimales."),
	e("BR-FR-DEC-01_BT-137_BT-142", "Les bases des remises et charges de ligne (BT-137, BT-142) ont au plus 19 chiffres dont 2 décimales."),
	// BR-FR-DEC-02.
	e("BR-FR-DEC-02_BT-129", "La quantité facturée (BT-129) a au plus 19 chiffres dont 4 décimales."),
	e("BR-FR-DEC-02_BT-149", "La quantité de base du prix (BT-149) a au plus 19 chiffres dont 4 décimales."),
	// BR-FR-DEC-03.
	e("BR-FR-DEC-03_BT-146", "Le prix net (BT-146) est positif (sauf cadres S9, B9, M9) avec au plus 19 chiffres dont 6 décimales."),
	e("BR-FR-DEC-03_BT-147", "Le rabais sur prix (BT-147) est positif (sauf cadres S9, B9, M9) avec au plus 19 chiffres dont 6 décimales."),
	e("BR-FR-DEC-03_BT-148", "Le prix brut (BT-148) est positif (sauf cadres S9, B9, M9) avec au plus 19 chiffres dont 6 décimales."),
	// BR-FR-DEC-04.
	e("BR-FR-DEC-04_BT-96_BT-103", "Le taux de TVA des remises et charges (BT-96, BT-103) est positif avec au plus 4 chiffres dont 2 décimales."),
	e("BR-FR-DEC-04_BT-119", "Le taux de TVA de la ventilation (BT-119) est positif avec au plus 4 chiffres dont 2 décimales."),
	e("BR-FR-DEC-04_BT-152", "Le taux de TVA de la ligne (BT-152) est positif avec au plus 4 chiffres dont 2 décimales."),
	// BR-FR-MV-01.
	e("BR-FR-MV-01_EXT-FR-FE-163", "Cadres S8, B8, M8, S9, B9, M9 : chaque ligne a un sous-type de ligne (EXT-FR-FE-163) renseigné."),
	// BR-FR-MV-02.
	e("BR-FR-MV-02_EXT-FR-FE-163", "Cadres S8, B8, M8 : la facture a au moins une ligne GROUP sans parent."),
	// BR-FR-BD-02.
	e("BR-FR-BD-02_EXT-FR-FE-163", "Cadres S9, B9, M9 : la facture a exactement deux lignes GROUP sans parent."),
	e("BR-FR-BD-02_BT-30", "Cadres S9, B9, M9 : exactement une ligne GROUP sans parent a pour vendeur de ligne le vendeur (BT-30)."),
	e("BR-FR-BD-02_BT-47", "Cadres S9, B9, M9 : exactement une ligne GROUP sans parent a pour vendeur de ligne l'acheteur (BT-47)."),
	// BR-FR-MV-03.
	e("BR-FR-MV-03_EXT-FR-FE-164", "Une ligne GROUP sans parent indique le nom du vendeur de ligne (EXT-FR-FE-164)."),
	e("BR-FR-MV-03_EXT-FR-FE-167", "Une ligne GROUP sans parent indique l'identifiant légal du vendeur de ligne (EXT-FR-FE-167)."),
	e("BR-FR-MV-03_EXT-FR-FE-177", "Une ligne GROUP sans parent indique le pays du vendeur de ligne (EXT-FR-FE-177)."),
	e("BR-FR-MV-03_EXT-FR-FE-181", "Une ligne GROUP sans parent indique son total de TVA en devise de facture (EXT-FR-FE-181)."),
	e("BR-FR-MV-03_EXT-FR-FE-182", "Une ligne GROUP sans parent indique son total de TVA en devise de comptabilité (EXT-FR-FE-182) si BT-6 est présent."),
	e("BR-FR-MV-03_BT-128_AFL", "Une ligne GROUP sans parent a un objet facturé (BT-128) de type 130 et de schéma AFL renseigné."),
	e("BR-FR-MV-03_BT-128_AVV", "Une ligne GROUP sans parent a un objet facturé (BT-128) de type 130 et de schéma AVV renseigné, autre que M8, S8, B8."),
	// BR-FR-MV-05.
	e("BR-FR-MV-05_EXT-FR-FE-BG-12", "Le total HT d'une ligne GROUP sans parent est égal à la somme de ses sous-lignes (±0,01 par sous-ligne)."),
	// BR-FR-MV-06.
	e("BR-FR-MV-06_EXT-FR-FE-167", "Chaque ligne a l'identifiant légal du vendeur de ligne de sa ligne parent."),
	// BR-FR-MV-07.
	e("BR-FR-MV-07_BT-128", "Chaque ligne a le numéro de sous-facture AFL de sa ligne parent."),
	// BR-FR-MV-08.
	e("BR-FR-MV-08_BT-128", "La raison d'exemption de chaque ligne DETAIL commence par #numéro de sous-facture#."),
	// BR-FR-MV-09.
	e("BR-FR-MV-09_EXT-FR-FE-181", "Le total de TVA d'une ligne GROUP sans parent est égal à la somme des ventilations de TVA de sa sous-facture (±0,01)."),
	// BR-FR-MV-10.
	e("BR-FR-MV-10_EXT-FR-FE-184", "Le total TTC d'une ligne GROUP sans parent est égal à son total HT plus sa TVA (±0,01 par sous-ligne)."),
	// BR-FR-MV-11.
	e("BR-FR-MV-11_BT-128", "La sous-facture du vendeur principal porte le numéro de facture (BT-1)."),
	// BR-FR-MV-12.
	e("BR-FR-MV-12_BT-128", "Les numéros de sous-facture des lignes GROUP sans parent sont uniques."),
	// BR-FR-MV-13.
	e("BR-FR-MV-13_BT-3", "Cadres S8, B8, M8 : le type de facture n'est pas auto-facturé."),
	// BR-FR-BD-13.
	e("BR-FR-MV-BD_BT-3", "Cadres S9, B9, M9 : le type de facture est auto-facturé (pattern BR-FR-BD-13, identifié BR-FR-MV-BD_BT-3 dans le schématron UBL)."),
	// BR-FR-MV-14.
	e("BR-FR-MV-14-EXT-FR-FE-136", "Cadres S8, B8, M8 : les lignes GROUP sans parent des rectificatives et avoirs référencent une facture antérieure (EXT-FR-FE-136)."),
	e("BR-FR-MV-14-EXT-FR-FE-138", "Cadres S8, B8, M8 : les lignes GROUP sans parent des rectificatives et avoirs donnent la date de la facture antérieure (EXT-FR-FE-138)."),
	// BR-FR-BD-14.
	e("BR-FR-BD-14-EXT-FR-FE-136", "Cadres S9, B9, M9 : les lignes GROUP sans parent des rectificatives et avoirs référencent une facture antérieure (EXT-FR-FE-136)."),
	e("BR-FR-BD-14-EXT-FR-FE-138", "Cadres S9, B9, M9 : les lignes GROUP sans parent des rectificatives et avoirs donnent la date de la facture antérieure (EXT-FR-FE-138)."),
}
