# Metro CCDC Scoreboard — Individual

Scoreboard para medir el **uptime de los servicios de cada participante**. Cada estudiante
tiene sus propias IPs y su propia lista de servicios; el dashboard muestra una fila por
participante con el estado de cada uno de sus servicios, más injects por participante.

Es una copia independiente de `metrosco/` (el scoreboard por equipos), que queda intacto.
Backend en Rust (axum), frontend en React + TypeScript + Tailwind, checkers en bash/python.

```
┌─────────── Online ───────────┐ ┌────── Leaderboard ──────┐
│ ● Minh    172.16.101.90      │ │ 1. Minh:   15           │
│ ● Nikita  172.16.101.58      │ │ 2. Nikita:  9           │
└──────────────────────────────┘ └─────────────────────────┘
Participant   Services                              Uptime   Score
Minh          [SSH] [SMTP] [WEB] [HTTPS] [IMAP]     100.0%   15
Nikita        [DNS] [FTP] [HTTPS] [WEB]              75.0%    9
Inject Schedule
```

---

## Índice
1. [Requisitos](#requisitos)
2. [Arrancar](#arrancar)
3. [Participantes (`teams.yaml`)](#participantes-teamsyaml)
4. [Servicios (`services.yaml`)](#servicios-servicesyaml)
5. [Qué verifica cada chequeo y cada cuánto](#qué-verifica-cada-chequeo-y-cada-cuánto)
6. [El dashboard](#el-dashboard)
7. [Página del participante](#página-del-participante)
8. [Injects (`injects.csv`)](#injects-injectscsv)
9. [Página de admin](#página-de-admin)
10. [Guardado y autoguardado](#guardado-y-autoguardado)
11. [Contraseñas de servicios (`PW/`)](#contraseñas-de-servicios-pw)
12. [Variables de entorno](#variables-de-entorno)
13. [Escribir checkers nuevos](#escribir-checkers-nuevos)
14. [Desarrollo](#desarrollo)
15. [Limitaciones conocidas](#limitaciones-conocidas)

---

## Requisitos
- Linux con `bash` en `/bin/bash`.
- Rust (`cargo`) para el backend.
- Node/npm solo si vas a modificar el frontend (el build ya viene en `public/`).
- Herramientas que usan los checkers:

| Herramienta | Paquete Debian/Ubuntu | La usa |
|---|---|---|
| `curl` | `curl` | WEB, HTTPS |
| `dig` | `dnsutils` | DNS |
| `ping` | `iputils-ping` | Panel "Online" |
| `ldapsearch` | `ldap-utils` | AD |
| `nc` | `netcat-openbsd` | `port.sh` |
| `python3` | `python3` | checkers de correo con login |

SSH, SMTP, IMAP, FTP y POP3 usan `banner.sh`, que solo necesita bash.

## Arrancar
```bash
cd individual
cargo run -r
```
Abre **http://localhost:8001**. El puerto por defecto es 8001 para poder correrlo al mismo
tiempo que `metrosco/` (8000). Se cambia con `SB_PORT`.

El juego arranca **activo**: desde que el servidor inicia ya se están verificando los
servicios y corre el reloj de injects. Se puede pausar/reanudar desde `/admin`.

Con Docker:
```bash
docker build -t scoreboard-individual .
docker run -p 8001:8001 scoreboard-individual
```

---

## Participantes (`teams.yaml`)
Archivo: `resources/teams.yaml`. Cada bloque es un participante. Todas las variables del
bloque se pasan como variables de entorno a los checkers.

```yaml
Minh:
  SERVICES: "SSH,SMTP,WEB,HTTPS,IMAP"
  IP: "172.16.101.90"
  SSH_HOST: "172.16.101.90"
  SMTP_HOST: "172.16.101.90"
  IMAP_HOST: "172.16.101.90"
  WEB_URL: "http://172.16.101.90"
  HTTPS_URL: "https://172.16.101.90"

Nikita:
  SERVICES: "FTP,WEB,HTTPS,DNS"
  IP: "172.16.101.58"
  FTP_HOST: "172.16.101.58"
  WEB_URL: "http://172.16.101.58"
  HTTPS_URL: "https://172.16.101.58"
  DNS_SERVER: "172.16.101.58"
  DNS_RECORD: "ccdcteam.com"
```

### Variables especiales
| Variable | Qué hace |
|---|---|
| `SERVICES` | Servicios que se le verifican, separados por coma o espacio (`"AD,DNS,FTP,WEB"` o `"AD DNS FTP WEB"`). Deben coincidir con los nombres de `services.yaml` (no importan mayúsculas). A los demás servicios **no se les ejecuta el checker** y no aparecen en su fila. **Si se omite, se le verifican todos.** |
| `IP` | Opcional. IP que aparece en el panel "Online" y a la que se le hace ping. Sin `IP`, el participante no aparece en "Online". |
| `TEAM_PASSWORD` | Opcional. Contraseña para entrar a su página (injects). Sin ella, cualquiera puede abrir `/team/<nombre>`. |

### Variables por servicio
| Servicio | Variables | Por defecto |
|---|---|---|
| AD | `AD_HOST` | |
| DNS | `DNS_SERVER`, `DNS_RECORD` | |
| FTP | `FTP_HOST`, `FTP_PORT` | puerto 21 |
| WEB | `WEB_URL` (ej. `http://1.2.3.4`) | |
| HTTPS | `HTTPS_URL` (ej. `https://1.2.3.4`) | |
| SSH | `SSH_HOST`, `SSH_PORT` | puerto 22 |
| SMTP | `SMTP_HOST`, `SMTP_PORT` | puerto 25 |
| IMAP | `IMAP_HOST`, `IMAP_PORT` | puerto 143 |
| POP3 | `POP3_HOST`, `POP3_PORT` | puerto 110 |

Las variables también sirven dentro del texto de los injects (ver [Injects](#injects-injectscsv)).

> ⚠️ No subas al repo IPs reales del evento ni credenciales. Mantén tu `teams.yaml` real
> solo en local.

---

## Servicios (`services.yaml`)
Archivo: `resources/services.yaml`. Es el **catálogo**: nombre del servicio → comando bash
que lo verifica. Cada participante elige cuáles le aplican con `SERVICES`.

```yaml
AD: AD/nologin.sh $AD_HOST
DNS: DNS/lookup.sh $DNS_RECORD $DNS_SERVER
FTP: ./banner.sh $FTP_HOST ${FTP_PORT:-21} '^220'
WEB: WEB/http_up.sh $WEB_URL
HTTPS: WEB/http_up.sh $HTTPS_URL
SSH: ./banner.sh $SSH_HOST ${SSH_PORT:-22} '^SSH-'
SMTP: ./banner.sh $SMTP_HOST ${SMTP_PORT:-25} '^220'
IMAP: ./banner.sh $IMAP_HOST ${IMAP_PORT:-143} '^\* OK'
POP3: ./banner.sh $POP3_HOST ${POP3_PORT:-110} '^\+OK'
```

- Los comandos se ejecutan con `bash -c` desde la carpeta `resources/`, con las variables
  del participante como entorno (más `PATH`, nada más).
- `${VAR:-x}` usa `x` si el participante no define `VAR`.
- Exit code `0` = arriba; cualquier otro = caído.
- También existe el formato largo:
  ```yaml
  WEB:
    command: WEB/http_up.sh $WEB_URL
    multiplier: 2
  ```
  (`multiplier` se guarda y se edita en admin, pero hoy cada check exitoso suma 1 punto
  sin importar su valor.)

Al final del archivo hay versiones **con login** comentadas (`FTP_LOGIN`, `SSH_LOGIN`,
`SMTP_AUTH`, `POP3S`) para cuando se quiera verificar autenticación real.

---

## Qué verifica cada chequeo y cada cuánto

### Frecuencia
| Qué | Cada cuánto |
|---|---|
| Verificación de servicios | **10 s**, todos los participantes y servicios en paralelo |
| Tiempo máximo por check | **5 s**; si no termina, cuenta como caído |
| Refresco del dashboard | 5 s (en el navegador) |
| Ping del panel "Online" | 5 s, lo dispara cada navegador que tenga la página abierta |
| Autoguardado | 10 min |

### Tipo de chequeo
| Servicio | Cómo se verifica | Pasa si… |
|---|---|---|
| SSH | TCP al 22, lee la primera línea | empieza con `SSH-` |
| SMTP | TCP al 25, lee la primera línea | empieza con `220` |
| IMAP | TCP al 143, lee la primera línea | empieza con `* OK` |
| FTP | TCP al 21, lee la primera línea | empieza con `220` |
| POP3 | TCP al 110, lee la primera línea | empieza con `+OK` |
| WEB / HTTPS | `curl` a la URL (acepta certificados autofirmados) | código HTTP < 500 (200, 301, 403… pasan) |
| DNS | `dig @DNS_SERVER DNS_RECORD` | el servidor responde algo, **aunque sea NXDOMAIN** |
| AD | `ldapsearch` anónimo a `ldap://AD_HOST` | el servidor LDAP responde (falla solo si no se puede contactar) |

Son chequeos de **disponibilidad**: confirman que el servicio está escuchando y responde
como ese protocolo. No detectan autenticación rota, una página web cambiada o un registro
DNS borrado. Para algo más estricto usa los checkers con login o `WEB/curlfind.sh`
(busca un texto en la página).

### Uptime y score
- Cada check exitoso suma **1 punto** al participante.
- **Uptime de un servicio** = checks exitosos / checks totales de ese servicio.
- **Uptime del participante** (columna Uptime) = promedio del uptime de sus servicios.
- El historial guarda los **últimos 10** resultados (~100 s) de cada servicio.

---

## El dashboard
`http://localhost:8001/`

1. **Online** (arriba a la izquierda): ping ICMP a la `IP` de cada participante.
   Azul = responde, rojo = no responde en 1 s. No suma puntos; sirve para distinguir
   "máquina apagada o sin red" de "servicio caído". Ojo: Windows y muchos firewalls
   bloquean ping por defecto, así que puede salir rojo aunque los servicios estén bien.
2. **Leaderboard** (arriba a la derecha): participantes ordenados por score.
3. **Tabla de participantes**: una fila por participante con
   - un recuadro por servicio asignado: **verde** = arriba, **rojo** = caído,
     **gris** = aún no verificado, con su % de uptime. Al pasar el mouse se ve la
     descripción del check y los últimos resultados (▲ arriba / ▼ caído, el más nuevo primero);
   - `n/m up`: cuántos de sus servicios están arriba ahora;
   - uptime promedio y score.

   El nombre enlaza a la página del participante.
4. **Inject Schedule**: horario de injects con cuenta regresiva. Ver
   [Injects](#injects-injectscsv).

---

## Página del participante
`http://localhost:8001/team/<Nombre>` (el nombre distingue mayúsculas).

- Historial de cada uno de **sus** servicios y su score total.
- **Active Injects**: injects activos, con cuánto falta para la entrega; al abrir uno se
  ve su descripción y se sube la respuesta.
- **Submission History**: lo que ya entregó (marca `(Late)` si fue tarde).
- **Passwords**: cambiar las contraseñas de servicios (ver [PW](#contraseñas-de-servicios-pw)).

Si el participante tiene `TEAM_PASSWORD`, primero debe entrar en `/login` con su nombre y
esa contraseña.

---

## Injects (`injects.csv`)
Archivo: `resources/injects.csv`. Cada fila es un inject y aplica a **todos** los
participantes; cada uno entrega su propia respuesta.

```csv
Start,Inject,Duration
00:01,BCOM05T - Memo Outlining DRP Resource Needs,30
00:10,TOOL07T - Perimeter Firewall Setup,22
```

| Columna | Obligatoria | Qué es |
|---|---|---|
| `Start` | sí | Minuto del juego en que empieza (`HH:MM` o minutos, ej. `90`). |
| `Inject` | sí | Nombre. Con formato `ID - Título` se genera una descripción por defecto con el ID. |
| `Duration` | no | Minutos que dura (mínimo 1). Si falta, se puede usar `End`. |
| `End` | no | Alternativa a `Duration` (`HH:MM` o minutos). |
| `Markdown` | no | Descripción en Markdown. Si falta: título + "Please submit the requested report." |
| `File Types` | no | Extensiones aceptadas, separadas por coma (`pdf,docx`). Vacío = cualquiera. |
| `No Submit` | no | `true` = informativo, no se entrega nada. |
| `Sticky` | no | `true` = nunca termina (queda visible siempre). |
| `Side Effects` | no | Lista YAML de cambios a los servicios al **terminar** el inject (ver abajo). |

Las columnas opcionales se pueden omitir del encabezado. Si un campo tiene comas o saltos
de línea (Markdown, Side Effects), ponlo entre comillas dobles.

### Variables en el texto
El Markdown se procesa con Handlebars usando las variables del participante, así cada
uno ve sus propios datos:
```markdown
Configura el registro {{DNS_RECORD}} en tu servidor {{DNS_SERVER}}.
```

### Archivos descargables
Todo lo que pongas en `resources/downloads/` se sirve en `/downloads/...`, así que puedes
enlazarlo desde un inject: `[Plantilla](/downloads/plantilla.docx)`.

### Entregas
- Se guardan en `resources/injects/<Participante>/<Nombre_del_inject>_response.<ext>`.
- Si se entrega después de que terminó: `..._late_response.<ext>` y queda marcada como tardía.
- Un inject que pide entrega le sigue apareciendo al participante hasta que entregue.

### Side effects
Se aplican cuando el inject termina. Sirven para cambiar lo que se verifica a mitad del
juego (cada participante sigue viendo solo los servicios de su `SERVICES`):
```yaml
- !DeleteService SSH
- !AddService
    name: HTTPS
    command: WEB/http_up.sh $HTTPS_URL
    multiplier: 1
- !EditService
    - WEB
    - name: WEB
      command: WEB/curlfind.sh $WEB_URL "Bienvenido"
      multiplier: 1
```
Borrar un servicio elimina su historial y score; editarlo los conserva.

### Dos relojes
- **Reloj del juego** (backend): arranca con el servidor y se pausa/reanuda desde admin.
  Controla qué injects están activos en la página del participante, las entregas tardías
  y los side effects.
- **Inject Schedule** del dashboard: usa el botón **Start Competition**. Guarda la hora en
  que se presionó (solo se puede presionar una vez) y desde ahí muestra los injects
  conforme empiezan, con su cuenta regresiva.

Para que coincidan, resetea e inicia el juego desde admin y presiona **Start Competition**
al mismo tiempo.

---

## Página de admin
`http://localhost:8001/admin`

Permite, en caliente:
- **Controles**: iniciar/pausar el juego y resetear scores.
- **Servicios**: agregar, editar, borrar y **probar** un servicio (lo ejecuta contra los
  participantes que lo tienen asignado y muestra stdout/stderr).
- **Participantes**: agregar/borrar y editar sus variables, incluida `SERVICES` (el
  cambio aplica desde el siguiente check).
- **Injects**: crear, editar y borrar.
- **Saves**: guardar y cargar partidas.

Para protegerla define `SB_ADMIN_PASSWORD` y entra en `/login` con usuario `admin`.

> ⚠️ **Sin `SB_ADMIN_PASSWORD` la página de admin queda abierta para cualquiera.**

Los cambios hechos en admin no se escriben en los `.yaml`; usa **Saves** para no perderlos.

---

## Guardado y autoguardado
- Guardados manuales: `resources/save/<nombre>.json` (desde admin).
- Autoguardado cada 10 min en `resources/save/autosave/autosave-N.json` (rota entre 12 archivos).
- Incluyen participantes, variables, servicios, scores, historial, injects, entregas
  registradas y contraseñas de `PW/`.
- Al cargar un save el juego queda **pausado**; reanúdalo desde admin.
- `resources/save/` está en `.gitignore`.

## Contraseñas de servicios (`PW/`)
Opcional, para checkers que hacen login. Cada participante puede tener archivos
`resources/PW/<Participante>/<GRUPO>.pw` con líneas `usuario:contraseña`. Si existen, el
participante puede cambiarlas desde su página.

El nombre del participante **no** se pasa automáticamente a los checkers; agrégalo como
variable en `teams.yaml` (por ejemplo `PW_NAME: "Minh"`) y úsalo así:
```yaml
FTP_LOGIN: FTP/login.sh $FTP_HOST $(shuf -n 1 PW/$PW_NAME/FTP.pw)
```

---

## Variables de entorno
| Variable | Por defecto | Qué hace |
|---|---|---|
| `SB_PORT` | `8001` | Puerto HTTP (escucha en todas las interfaces). |
| `SB_RESOURCE_DIR` | `resources` | Carpeta de configuración y checkers (cwd de los checkers). |
| `SB_TEAMS` | `teams.yaml` | Archivo de participantes dentro de `SB_RESOURCE_DIR`. |
| `SB_SERVICES` | `services.yaml` | Catálogo de servicios. |
| `SB_INJECTS` | `injects.csv` | Archivo de injects. |
| `SB_APP_DIR` | `public` | Carpeta del frontend compilado. |
| `SB_ADMIN_PASSWORD` | *(sin definir)* | Contraseña de `admin`. |
| `LOG_LEVEL` | `scoreboard=info,tower_http=info` | Nivel de logs (`scoreboard=debug` muestra stdout/stderr de cada check). |

Ejemplo con otro archivo de participantes y contraseña de admin:
```bash
SB_TEAMS=participantes-evento.yaml SB_ADMIN_PASSWORD='cambia-esto' cargo run -r
```

---

## Escribir checkers nuevos
Cualquier programa ejecutable desde bash sirve. Reglas:
- `exit 0` si el servicio está bien; cualquier otro código si no.
- Debe terminar en menos de 5 s.
- Si falla, escribe el motivo en stdout/stderr (se ve al probar el servicio en admin y en
  los logs con `LOG_LEVEL=scoreboard=debug`).
- Credenciales como un solo argumento `usuario:contraseña` (compatible con `PW/`).
- Guárdalo en la carpeta del servicio (`WEB/`, `FTP/`, …) y agrégalo a `services.yaml`.

Checkers incluidos (rutas relativas a `resources/`):

| Checker | Uso |
|---|---|
| `banner.sh` | `<host> <puerto> <regex>`: pasa si la primera línea del servidor cumple la regex |
| `port.sh` | `<host> <puerto>`: puerto TCP abierto (requiere `nc`) |
| `WEB/http_up.sh` | `<url>`: responde con código < 500 |
| `WEB/curlfind.sh` | `<url> <texto> [--insecure]`: la página contiene el texto |
| `WEB/http_compare.sh` | `<url> <archivo_esperado> [--insecure]`: el cuerpo coincide con el archivo |
| `DNS/lookup.sh` | `<registro> [servidor]` |
| `AD/nologin.sh` | `<host>`: LDAP anónimo |
| `AD/login.sh` | `<host> <dominio> <usuario:contraseña> [directorio]` |
| `FTP/login.sh` | `<host> <usuario:contraseña> [ruta]` (requiere cliente `ftp`) |
| `SSH/login.sh`, `SSH/nologin.sh` | requieren `sshpass` |
| `MAIL/smtp_check.py` | `<host> <puerto> <usuario> <pass> <from> <to> [--starttls]`: envía un correo de prueba |
| `MAIL/pop3_check.py` | `<host> <puerto> <usuario> <pass> [--ssl]` |
| `MC/matchdesc.py` | `<host> <descripción>`: servidor de Minecraft |

Probar un checker a mano:
```bash
cd resources
./banner.sh 172.16.101.90 22 '^SSH-'; echo $?
```

---

## Desarrollo
```bash
cargo test                      # tests del backend
cd Client && npm ci             # dependencias del frontend (una vez)
npm run dev                     # Vite en modo desarrollo; usa el backend en :8001
npm run build                   # chequeo de TypeScript + build
cd .. && just buildspa          # compila el frontend y lo copia a public/
```
`just buildspa` conserva `public/injects.html` y `public/metrohaha.html`.

Estructura:
```
individual/
├── src/                 backend Rust (checker/ = lógica, router/ = API)
├── Client/src/          frontend React (Pages/, Components/, Hooks/)
├── public/              frontend compilado que sirve el backend
└── resources/           teams.yaml, services.yaml, injects.csv y checkers
```

API principal (`/api`): `scores`, `reachability`, `time`, `competition`,
`competition/injects`, `team/<nombre>/scores`, `team/<nombre>/injects`, `admin/...`.

---

## Limitaciones conocidas
- Los chequeos de banner/HTTP/DNS verifican disponibilidad, no funcionalidad completa.
- El ping de "Online" depende de que ICMP no esté bloqueado.
- Las sesiones de login se guardan en memoria: al reiniciar el servidor hay que volver a entrar.
- `multiplier` no afecta el score todavía.
