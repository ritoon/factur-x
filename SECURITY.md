# Politique de sécurité

## Versions prises en charge

Le projet est en v0 : les correctifs de sécurité sont publiés sur la branche `main` et dans une nouvelle version de la dernière série publiée.

| Version | Correctifs de sécurité |
|---|---|
| `main` | oui |
| v0.1.x | oui |

## Signaler une vulnérabilité

N'ouvrez pas d'issue publique. Utilisez le signalement privé de GitHub : onglet **Security**, bouton **Report a vulnerability**, ou directement [ce formulaire](https://github.com/ritoon/zerechnung/security/advisories/new). Le rapport n'est visible que par le mainteneur.

Indiquez si possible :

- la version ou le commit concerné, et la version de Go ;
- la fonction ou la commande en cause (`Read`, `Check`, `Embed`, `validate.XML`, `zerechnung validate`…) ;
- un fichier qui reproduit le problème (PDF ou XML), fictif ou anonymisé ;
- l'effet observé : plantage, consommation de mémoire ou de temps, données lues ou écrites à tort.

## Ce qui relève de cette politique

Le package lit des PDF et des XML venus de l'extérieur : les factures reçues. Relève de cette politique tout ce qu'un fichier hostile peut obtenir, par exemple :

- un `panic`, une boucle sans fin ou une récursion qui fait déborder la pile ;
- une consommation de mémoire ou de temps sans commune mesure avec la taille du fichier, malgré les bornes du lecteur PDF (256 Mio décompressés par flux, profondeur d'imbrication limitée) et du lecteur XML (256 niveaux, aucune entité développée hors des cinq prédéfinies) ;
- un PDF produit par `Embed` ou `Create` qui modifie les octets d'origine ou invalide une signature existante.

Une règle de validation levée à tort ou manquée est un bogue ordinaire : signalez-la par une [issue](https://github.com/ritoon/zerechnung/issues/new/choose), avec le modèle « Écart de validation ».

## Traitement

Vous recevez un accusé de réception sous 7 jours. Le correctif est préparé dans un avis de sécurité privé, puis publié avec une nouvelle version ; l'avis est rendu public à ce moment-là, en vous citant si vous le souhaitez.
