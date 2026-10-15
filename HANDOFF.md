# HANDOFF — unifi-provider
_Mis à jour : 2026-10-15 21:50_

## Objectif
Faire évoluer le provider Terraform/OpenTofu UniFi (`ubiquiti-community/terraform-provider-unifi`, fork `kallioli`) pour couvrir ce qui manque au réseau domestique de Kevin en infrastructure as code (projet voisin `~/work/unifi-network`, provider `~> 0.56`). Première cible, décidée par Kevin le 2026-10-15 : le réglage **SNMP / SNMPv3 du contrôleur** (Settings > System > SNMP), aujourd'hui posé à la main dans l'UI pour la supervision Zabbix.

## État
- [x] 2026-10-15 : état des lieux en lecture seule (ce fichier). Rien poussé, aucun test d'acceptation lancé.
- [x] Build : `go build ./...` OK (go 1.27.1 via mise, module `go 1.25.8`), ~20 s après téléchargement des dépendances.
- [x] `go vet ./...` OK. Tests unitaires `go test ./...` OK (paquets `unifi` et `unifi/validators`). Sans `TF_ACC`, les tests d'acceptation sont ignorés : aucun contrôleur contacté.
- [x] 2026-10-15 : réponse du lobby (lecture publique GitHub de son côté) : amont `main` = `647cf534`, dernier tag stable **v0.59.0** (v0.60.0-beta.0 en cours). Le `main` du fork (`ef0da40c`) a 4 mois de retard. **Aucun bloc `snmp` ni fichier snmp en amont** : le travail reste à faire.
- [ ] `git fetch upstream` : **non lancé**, github.com n'est pas au registre des accès (règle imposée : demander à Kevin avant toute connexion, même en lecture). À faire dès que Kevin ouvre l'accès (question a).
- [x] 2026-10-15 : **accès SSH en lecture à dev-kevin (`kallioli@10.25.0.19`) validé par Kevin dans la session**, dépôt `~/Projects/terraform-provider-unifi` (et `~/Projects/go-unifi`). Toutes les refs rapatriées en local : remote `devkevin` (9 branches + `main` = v0.59.0), `devkevin-upstream/*` (copie de l'amont au 2026-10-03, `bfdb8627`), tags jusqu'à v0.59.0.
- [x] **Aucun travail SNMP sur dev-kevin** : pas de branche, pas de commit, pas de stash, copie de travail propre, rien non plus dans go-unifi. Les 9 branches locales de Kevin (non poussées sur le fork) portent sur les défauts statiques, device config_network, port_forward `enabled` ; dernière activité 2026-10-03.
- [x] **Piste trouvée dans les notes de Kevin du 2026-10-03** : la PR amont **#485 (jaxzin)** ajoute `snmp` à `unifi_setting`. Bloquée par sa base (`feat/attribute-nesting`, PR #478, helpers `util.NestFields`/`WithObject`/`UpgradeRawState`), Kevin a demandé un rebase sur `main`. Réserve de Kevin : l'écho en lecture de `community`/`x_password` n'est vérifié que sur l'image de test, et **le réglage `snmp` est absent de `get/setting` sur un vrai UCG-Fiber 10.6.106** ; il a suggéré le motif write-only `password_wo`. État actuel de #485 demandé au lobby.
- [ ] Bloc `snmp` dans `unifi_setting` : à faire (voir Notes, « Où insérer SNMP »), sur une branche `claude/setting-snmp` basée sur **`upstream/main`** (en attendant l'accès GitHub : `devkevin-upstream/main`, 2026-10-03, à rafraîchir ensuite), pas sur le `main` du fork. Ne pas toucher au `main` local. Décision à prendre : repartir de #485 (si rebasée) ou écrire le bloc ici.
- [x] 2026-10-15 : **go de Kevin pour le conteneur de test**, lancé sur la VM (voir Notes « Contrôleur de test ») : image `docker.io/jacobalberty/unifi:latest` = UniFi Network **10.0.162**, mode simulation, admin/admin, `https://localhost:8443`. **Tourne encore** (`podman ps`), à arrêter avec `podman compose -f <scratchpad>/unifi-test-compose.yaml down` ou `podman rm -f unifi`.
- [x] **Sonde SNMP sur le conteneur** (curl puis go-unifi) : (1) la clé `snmp` est **absente de `get/setting` tant qu'aucun `set` n'a eu lieu**, puis présente ; cohérent avec la note de Kevin sur le UCG-Fiber ; `GetSetting[*settings.Snmp]` renvoie bien `NotFoundError` dans ce cas, le Create doit le tolérer (comme igmp). (2) `set/setting/snmp` avec `enabledV3`, `username`, `x_password` est accepté et **`x_password` est renvoyé en clair** par `get/setting` et `get/setting/snmp` sur ce contrôleur. (3) `UpdateSetting` + relecture avec le modèle généré `settings.Snmp` de go-unifi v1.33.43 fonctionnent sans modification de la bibliothèque. Reste inconnu : le comportement d'un vrai UniFi OS 10.x (UCG/UDM) pour la relecture du mot de passe.
- [x] Outillage complet sur la VM : go 1.27.1, golangci-lint 2.14.0 (installé via le proxy Go, shim mise, base à 0 problème), tfplugindocs (`go generate` sans écart ; il crée `docs/actions` et `docs/list-resources`, non suivis par l'amont, à supprimer), terraform 1.16.5, tofu 1.13.1, podman 5.8.2.

## Prochaine étape
1. Attendre Kevin : accès github.com (lecture au minimum) ou sa réponse directe sur la PR amont #485 : ouverte, rebasée, fermée ? Si elle est rebasée sur `main` et propre, la stratégie devient : la relire, la tester, et pousser l'amont à la fusionner (ou la reprendre avec l'accord de son auteur). Sinon, écrire le bloc ici.
2. Créer `claude/setting-snmp` depuis `devkevin-upstream/main` (puis `upstream/main` dès l'accès GitHub), ajouter le bloc `snmp` à `unifi_setting` en suivant le modèle `igmp_snooping` (commit `f6939506`), mot de passe en **write-only** (`password_wo`, motif suggéré par Kevin sur #485) plutôt que `Sensitive` + relecture, avec test unitaire de conversion.
3. Question c : tranchée sur le conteneur (clé absente avant le premier `set`, `x_password` relu en clair). Reste à Kevin un `GET /api/s/default/get/setting/snmp` en lecture sur son contrôleur réel pour confirmer la relecture du mot de passe sous UniFi OS ; sans confirmation, concevoir le bloc pour ne **pas** dépendre de la relecture (write-only ou valeur du plan conservée).
3. Mettre à jour `docs/resources/setting.md` par `go generate ./...` (tfplugindocs est déclaré en `tool` dans go.mod), plus `examples/resources/unifi_setting/resource.tf` et `CHANGELOG.md`.

## Blocages / questions pour Kevin
- **Accès GitHub absent du registre** : `origin` et `upstream` sont sur github.com. Il faut une entrée au registre (fetch/push de branches `claude/*` sur `kallioli/terraform-provider-unifi`, jamais sur `main`) et un moyen d'authentification (clé de déploiement avec la clé publique de l'agent, `/usr/local/lib/claude-vm/agent-ssh-key.pub`). Sans cela : pas de fetch de ta branche, pas de push, pas de PR.
- Version : **réglé par le lobby le 2026-10-15** (amont à v0.59.0, pas de snmp en amont, voir État). Il reste à faire le fetch une fois l'accès ouvert.
- **Mot de passe SNMPv3 en lecture** : le champ API est `x_password`. Reste à confirmer sur un vrai contrôleur si le GET le renvoie en clair (comme `x_passphrase` des WLAN) ou masqué. Cela décide de la stratégie anti-drift (lire la valeur ou garder celle du plan comme pour `radius.secret`). Test à faire uniquement sur un contrôleur de test, jamais sur `10.99.0.1`.
- **Validation sur contrôleur** : le conteneur de test sur la VM est accepté par le lobby, mais il faut le **go de Kevin** avant de lancer le conteneur (il télécharge `docker.io/jacobalberty/unifi:latest`, accès réseau sortant hors registre). Le mode simulation du conteneur suffit pour le test de relecture de `x_password` ; la validation « sur un vrai UDM » exigée par l'amont dans ses PR reste à faire par Kevin sur son réseau.
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

### Contrôleur de test (go de Kevin le 2026-10-15, en cours d'exécution)
Le `docker-compose.yaml` du dépôt ne démarre pas tel quel sur code-1 : podman exige un nom d'image qualifié (`docker.io/...`) et **le port 8080 de l'hôte est déjà pris**. Un compose de session, copie du fichier du dépôt avec l'image qualifiée et `18080:8080`, est dans le scratchpad de la session (`unifi-test-compose.yaml`) ; à recréer au besoin (le scratchpad ne survit pas à la session). Sonde curl : `snmp-probe.sh [get|set]` au même endroit ; sonde go-unifi : `gounifi-probe/`.
```sh
podman pull docker.io/jacobalberty/unifi:latest
podman compose -f <compose de session> up -d
curl -sk https://localhost:8443/manage/account/login -o /dev/null -w '%{http_code}\n'   # 200 après ~1 min
```
Authentification de l'API classique : `POST /api/login` `{"username":"admin","password":"admin"}`, cookie + en-tête `X-Csrf-Token` pour les écritures.
Puis, pour la relecture de `x_password` (question c), un `GET https://localhost:8443/api/s/default/get/setting` authentifié admin/admin, ou bien les tests d'acceptation sans relancer de conteneur :
```sh
UNIFI_SKIP_CONTAINER=1 UNIFI_API=https://localhost:8443 UNIFI_USERNAME=admin UNIFI_PASSWORD=admin UNIFI_INSECURE=true \
  TF_ACC=1 go test ./unifi -run 'TestAccSetting' -v -count 1 -timeout 30m
```
Sans `UNIFI_SKIP_CONTAINER`, `make testacc` lance lui-même le compose via testcontainers (utilisateur admin/admin, port 8443 mappé). Arrêt : `podman compose -f docker-compose.yaml down`.

### Pièges
- `main` local est peut-être en retard sur l'amont (voir question version) : vérifier avant de brancher.
- Ne jamais pointer un `terraform apply` ou un test d'acceptation sur `10.99.0.1` (production domestique). Le contrôleur DevStack `10.20.0.225` n'est pas un contrôleur UniFi.
- L'état tofu de unifi-network contient déjà des secrets en clair : le mot de passe SNMPv3 y finira aussi, c'est connu et assumé côté unifi-network.

### Demandes ouvertes (sessions)
- Lobby, 2026-10-15 : état des lieux + HANDOFF. Réponse envoyée le 2026-10-15 ; retour du lobby reçu (amont v0.59.0, consignes de branche, conteneur accepté). Questions (a) accès GitHub et (d) cible PR/release remontées à Kevin par le lobby : **en attente**.
- Vers le lobby, 2026-10-15 14:00 : demande de l'état de la PR amont #485 (snmp) et de #478. **Refusée par le lobby, à raison** : github.com hors registre pour lui aussi. Question remontée à Kevin par le lobby avec la demande d'accès github.com en lecture. **En attente de Kevin, pas de code avant.**
- Accès dev-kevin : validé par Kevin le 2026-10-15 en session, lecture seule, compte `kallioli`, dépôts `~/Projects/terraform-provider-unifi` et `~/Projects/go-unifi`. À inscrire au registre s'il doit durer.
