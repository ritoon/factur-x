## Ce que change cette pull request

<!-- Ce qui change, et pourquoi. Lien vers l'issue s'il y en a une : « Closes #… ». -->

## Références

<!--
Pour une règle ajoutée ou modifiée : artefact officiel, version, identifiant.
Pour un écart de validation : validateur de référence, version et sortie.
Sinon, supprimez cette section.
-->

## Vérifications

- [ ] `gofmt -l` ne signale rien ; `go vet ./...` et `go test -race ./...` passent.
- [ ] `golangci-lint run` ne signale rien.
- [ ] Les tests sur les artefacts officiels (`eval "$(conformance/fetch.sh)"` puis `go test ./...`) passent, si la pull request touche aux règles, aux schémas ou à la lecture et l'écriture du XML.
- [ ] Un test couvre le changement.
- [ ] Le README suit le changement (couverture, limites, nombres de règles), s'il y a lieu.
- [ ] Aucune dépendance n'est ajoutée à `go.mod`.
- [ ] Les fichiers de test sont fictifs ou issus d'un corpus public, source et licence indiquées (voir `testdata/NOTICE`).
