# HANDOFF — unifi-provider
_Mis à jour : 2026-10-15 12:30_

## Objectif
Faire évoluer le provider Terraform/OpenTofu UniFi (`ubiquiti-community/terraform-provider-unifi`, fork `kallioli`) pour couvrir ce qui manque au réseau domestique de Kevin en infrastructure as code (projet voisin `~/work/unifi-network`, provider `~> 0.56`). Première cible, décidée par Kevin le 2026-10-15 : le réglage **SNMP / SNMPv3 du contrôleur** (Settings > System > SNMP), aujourd'hui posé à la main dans l'UI pour la supervision Zabbix.

## État
- [x] 2026-10-15 : état des lieux en lecture seule (ce fichier). Rien poussé, aucun test d'acceptation lancé.
- [x] Build : `go build ./...` OK (go 1.27.1 via mise, module `go 1.25.8`), ~20 s après téléchargement des dépendances.
- [x] `go vet ./...` OK. Tests unitaires `go test ./...` OK (paquets `unifi` et `unifi/validators`). Sans `TF_ACC`, les tests d'acceptation sont ignorés : aucun contrôleur contacté.
- [ ] Récupérer le travail en cours de Kevin (poste dev-kevin, à pousser sur une branche du fork) : en attente.
- [ ] Bloc `snmp` dans `unifi_setting` : à faire (voir Notes, « Où insérer SNMP »).

## Prochaine étape
1. Quand Kevin a poussé sa branche sur le fork : `git fetch origin`, lire la branche, comparer avec le plan ci-dessous, reprendre dessus plutôt que repartir de zéro.
2. Sinon, démarrer `claude/setting-snmp` depuis `main` : ajouter le bloc `snmp` à `unifi_setting` en suivant le modèle `igmp_snooping` (commit `f6939506`, le plus récent et le plus propre), avec test unitaire de conversion.
3. Mettre à jour `docs/resources/setting.md` par `go generate ./...` (tfplugindocs est déclaré en `tool` dans go.mod), plus `examples/resources/unifi_setting/resource.tf` et `CHANGELOG.md`.

## Blocages / questions pour Kevin
- **Accès GitHub absent du registre** : `origin` et `upstream` sont sur github.com. Il faut une entrée au registre (fetch/push de branches `claude/*` sur `kallioli/terraform-provider-unifi`, jamais sur `main`) et un moyen d'authentification (clé de déploiement avec la clé publique de l'agent, `/usr/local/lib/claude-vm/agent-ssh-key.pub`). Sans cela : pas de fetch de ta branche, pas de push, pas de PR.
- **Numéro de version** : le CHANGELOG local s'arrête à v0.44.0 (2026-06-10) alors que unifi-network épingle `~> 0.56` (0.56.1 installé). Aucun tag n'est présent dans le clone. Soit l'amont a avancé depuis le 10/06 (le fork a 4 mois de retard), soit le registre numérote autrement. À vérifier avec un `git fetch upstream --tags` une fois l'accès ouvert. Impact : la base de travail est peut-être périmée et l'amont a peut-être déjà un bloc `snmp`.
- **Mot de passe SNMPv3 en lecture** : le champ API est `x_password`. Reste à confirmer sur un vrai contrôleur si le GET le renvoie en clair (comme `x_passphrase` des WLAN) ou masqué. Cela décide de la stratégie anti-drift (lire la valeur ou garder celle du plan comme pour `radius.secret`). Test à faire uniquement sur un contrôleur de test, jamais sur `10.99.0.1`.
- **Validation sur contrôleur réel** : l'amont exige une validation (« Validated on a real UDM ») dans ses PR. Il faut décider où : conteneur `jacobalberty/unifi` via podman sur la VM (compose présent dans le dépôt, testé sous Podman d'après les commentaires), ou Kevin lui-même sur son réseau.
- **Destination** : PR vers l'amont `ubiquiti-community` (comme pour igmp_snooping #229, d'abord livré par Kevin en amont) ou release sur le fork seulement ? La release passe par GoReleaser + signature GPG (`.github/workflows/release.yaml`), donc par Kevin.

## Notes
### Le dépôt
- Fork GitHub de `ubiquiti-community/terraform-provider-unifi`. Remote `origin` = `kallioli/terraform-provider-unifi`, `upstream` = amont. Une seule branche `main`, alignée sur l'amont au 2026-06-10 (`ef0da40c`). Kevin contribue déjà en amont : `f6939506` « feat(setting): manage site-level igmp_snooping (#229) » est de lui.
- Go, terraform-plugin-framework (pas le SDK v2). Client API : `github.com/ubiquiti-community/go-unifi` (v1.33.43 du 2026-06-10). Les modèles de réglages sont dans `go-unifi/unifi/settings/`, générés depuis `ace.jar` (`*.generated.go`) sauf `igmp_snooping.go`, écrit à la main.
- Arborescence : `main.go` (serveur du provider, `go:generate` pour tfplugindocs), `unifi/` (une paire `<ressource>_resource.go` + `_test.go` par ressource, `provider.go` enregistre les ressources), `unifi/util`, `unifi/validators`, `docs/` (généré), `examples/`, `templates/`, `scripts/init.d/demo-mode` + `docker-compose.yaml` (contrôleur de test en mode démo pour les tests d'acceptation, testcontainers).
- CI GitHub Actions : `ci.yaml`, `lint.yaml` (golangci-lint v2, config `.golangci.yaml`), `acctest.yaml` (matrice de versions UniFi, `make testacc`), `release.yaml` (GoReleaser sur tag, clé GPG dans les secrets du dépôt amont). `golangci-lint` n'est pas installé sur la VM : `mise use -g golangci-lint@latest` si besoin.
- `make testacc` = `TF_ACC=1 go test …` : lance un contrôleur dans un conteneur. Ne pas le lancer sans décision de Kevin (podman disponible, mais image à télécharger, ~1 h en CI).

### Comment le provider gère les réglages de site
Il n'y a **pas** de famille `unifi_setting_*` : une seule ressource **`unifi_setting`** (`unifi/setting_resource.go`, 2 253 lignes) porte des blocs imbriqués optionnels (`SingleNestedAttribute`) : `doh`, `ips`, `mgmt`, `radius`, `usg`, `igmp_snooping`. ID = nom du site. Mécanique :
- `Create` et `Update` : pour chaque bloc non nul, conversion modèle → `settings.X` puis `r.client.UpdateSetting(ctx, site, setting)` (PUT `/api/s/<site>/set/setting/<clé>`), puis relecture par `readSettings`.
- `Read` et `readSettings` : ne lit que les blocs présents dans le plan ou l'état (`ui.GetSetting[*settings.X]`), met les autres à null.
- `Delete` : no-op (un réglage ne se supprime pas, il est juste retiré de l'état). `ImportState` : passthrough de l'ID.
- Deux styles de conversion : écrasement complet (`doh`, `ips`, `usg`) ou **lecture-modification-écriture** (`radius`, `igmp_snooping`) : on relit le réglage courant, on superpose seulement les champs posés dans le plan, pour ne pas écraser ce que l'UI a configuré. C'est le style à suivre.
- Secrets : `radius.secret` est `Sensitive`, `Optional + Computed` ; à la relecture on ne le renvoie dans l'état que si le plan le posait.

### Où insérer SNMP
`go-unifi` a déjà le modèle, clé API `snmp` (`settings/snmp.generated.go`, reconnu par `GetSettingKey`) :
```go
type Snmp struct {
    BaseSetting
    Community string `json:"community,omitempty"` // .{1,256}
    Enabled   bool   `json:"enabled"`
    EnabledV3 bool   `json:"enabledV3"`
    Password  string `json:"x_password,omitempty"` // [^'"]{8,32}
    Username  string `json:"username,omitempty"`   // [a-zA-Z0-9_-]{1,30}
}
```
Donc **aucune modification de go-unifi** n'est nécessaire a priori (à confirmer face au JSON réel du contrôleur : l'UI SNMPv3 ne montre qu'un utilisateur et un mot de passe, cohérent avec ce modèle). Tout se passe dans `unifi/setting_resource.go`, en copiant les six points de `igmp_snooping` :
1. `settingResourceModel` : champ `Snmp types.Object \`tfsdk:"snmp"\`` ; nouveau `settingSnmpModel` (`enabled`, `community`, `v3_enabled`, `username`, `password`) et `snmpAttrTypes`.
2. `Schema` : bloc `"snmp": schema.SingleNestedAttribute{Optional: true, …}` ; `password` en `Sensitive: true` ; validateurs regex calqués sur les contraintes de go-unifi (mot de passe 8 à 32 caractères sans guillemets, utilisateur `[a-zA-Z0-9_-]{1,30}`).
3. `Create` et `Update` : bloc « if !data.Snmp.IsNull() » avec `GetSetting[*settings.Snmp]` comme base (tolérer `NotFoundError`), `snmpModelToSetting`, `UpdateSetting`.
4. `readSettings` : lecture + `snmpSettingToModel`, sinon `types.ObjectNull(snmpAttrTypes)`. Passer le plan pour le mot de passe, comme `radiusSettingToModel`.
5. Helpers `snmpModelToSetting(ctx, model, base)` et `snmpSettingToModel(ctx, setting, plan)`.
6. Test unitaire `unifi/setting_snmp_test.go` sur le modèle de `TestIgmpSnoopingModelMerge` (sans contrôleur) ; test d'acceptation `TestAccSetting_snmp` dans `setting_resource_test.go` si on lance la CI amont.
Puis `go generate ./...` (docs), `examples/resources/unifi_setting/resource.tf`, `CHANGELOG.md`, `go vet`, `golangci-lint`.

### Pièges
- `main` local est peut-être en retard sur l'amont (voir question version) : vérifier avant de brancher.
- Ne jamais pointer un `terraform apply` ou un test d'acceptation sur `10.99.0.1` (production domestique). Le contrôleur DevStack `10.20.0.225` n'est pas un contrôleur UniFi.
- L'état tofu de unifi-network contient déjà des secrets en clair : le mot de passe SNMPv3 y finira aussi, c'est connu et assumé côté unifi-network.

### Demandes ouvertes (sessions)
- Lobby, 2026-10-15 : état des lieux + HANDOFF (cette tâche). Réponse envoyée le 2026-10-15.
