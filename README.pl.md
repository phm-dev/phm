# PHM - Menedżer PHP dla macOS

> **Zastrzeżenie:** To oprogramowanie jest dostarczane „takie jakie jest", bez jakiejkolwiek gwarancji, wyraźnej lub dorozumianej. Autorzy nie ponoszą odpowiedzialności za jakiekolwiek szkody wynikające z użytkowania tego oprogramowania. Używasz na własne ryzyko.

## Instalacja

```bash
curl -fsSL https://raw.githubusercontent.com/phm-dev/phm/main/scripts/install-phm.sh | bash
```

Po instalacji dodaj do swojego profilu powłoki (`~/.zshrc` lub `~/.bashrc`):

```bash
export PATH="/opt/php/bin:$PATH"
```

## Szybki start

```bash
# Aktualizuj indeks pakietów
phm update

# Zainstaluj PHP 8.5 z rozszerzeniami
phm install php8.5-cli php8.5-fpm php8.5-redis

# Ustaw jako domyślną wersję
phm use 8.5

# Sprawdź
php -v

# Tryb interaktywny (kreator)
phm ui
```

## Komendy

```bash
phm install <pakiet>     # Instaluj pakiety
phm remove <pakiet>      # Usuń pakiety
phm upgrade              # Aktualizuj wszystkie pakiety
phm list                 # Lista dostępnych pakietów
phm search <fraza>       # Szukaj pakietów
phm info <pakiet>        # Pokaż szczegóły pakietu
phm use <wersja>         # Ustaw domyślną wersję PHP
phm fpm start|stop|...   # Zarządzaj usługą PHP-FPM
phm ui                   # Interaktywny kreator
```

---

# 🜂 PHM — PHP Manager Mrocznych Krain

**Języki:**
- [English](README.md)
- Polski (ten plik)

> *W epoce nieskończonych rekompilacji,*  
> *gdy programiści spalali cykle CPU niczym kadzidło,*  
> *a łańcuchy zależności rosły dłuższe niż elfickie genealogie —*  
> *potrzebna była inna droga.*

---

## 🍎 Zakres Krainy

PHM **działa wyłącznie na macOS**.

Nie dlatego, że inne światy są gorsze —  
lecz dlatego, że ta opowieść dotyczy **konkretnej krainy**, w której:

- Homebrew stał się nieformalnym standardem,
- rekompilacja PHP na laptopach jest normą,
- binarne pakiety PHP po prostu **nie istnieją**.

PHM powstał, aby rozwiązać **realny problem macOS**:  
brak prostego, systemowego sposobu instalacji PHP w stylu:

```bash
apt install php8.5-cli
```

Linux ma swoje repozytoria.  
Debian ma Ondřeja.  
macOS — miał tylko kuźnie.

PHM wypełnia tę lukę.

---

## 🌑 Wiek Mroku

Niegdyś, w krainach macOS i Linuxa,  
programiści kompilowali PHP **w kółko**.

Na laptopach.  
Na runnerach CI.  
Na serwerach buildów.  
Na maszynach, które chciały jedynie uruchomić `php -v`.

Każdego dnia:

- te same źródła,
- te same flagi,
- te same rozszerzenia,
- te same błędy,
- te same stracone godziny,
- to samo CO₂ unoszące się po cichu z serwerowni i wentylatorów.

> *Dziesięć maszyn,*  
> *dziesięć buildów,*  
> *dziesięć nieco innych binarek,*  
> *żadna w pełni powtarzalna.*

Był to **Wiek Chaosu Zależności**.

Zaklęcia Homebrew wchodziły ze sobą w konflikt.  
Czary ASDF pękały bez ostrzeżenia.  
Grimuar phpenv gnił, pełen przestarzałych inkantacji.  
A każdy programista płacił cenę —  
czasem, zdrowiem psychicznym i watami.

---

## 🜃 Klątwa Rekomplikacji

PHP nie jest lekką magią.

Ciągnie za sobą:

- OpenSSL
- ICU
- libxml
- libzip
- rabbitmq-c
- zlib
- iconv
- i niezliczone inne byty

Skompilować PHP raz — to rozsądne.  
Skompilować je **wszędzie** — to szaleństwo.

A jednak świat uznał to szaleństwo za normę.

> *„Po prostu przebuduj lokalnie.”*  
> *„Po prostu użyj brew.”*  
> *„Po prostu spróbuj jeszcze raz.”*

I tak kuźnie płonęły dalej.

---

## ✨ Światło z Północy

W krainach Debiana pojawił się inny wzorzec.

Cichy mistrz-kowal imieniem **Ondřej**  
wykuł PHP **raz** —  
i podzielił się artefaktami ze światem.

Nie zaklęcia.  
Nie drzewa źródeł.  
**Pakiety.**

Powtarzalne.  
Przewidywalne.  
Instalowalne.

```bash
apt install php8.2-cli
apt install php8.2-fpm
apt install php8.2-redis
```

Bez rekompilacji.  
Bez niespodzianek.  
Bez marnowania ognia.

> *Jeden build.*  
> *Tysiące instalacji.*  
> *Zdrowy świat.*

---

## 🌿 PHM Podąża Starożytnym Wzorcem

**PHM** jest naszą odpowiedzią dla współczesnych krain.

Nie managerem wersji.  
Nie systemem buildów.  
Nie kolejną iluzją zależności.

PHM to **menedżer pakietów PHP**, inspirowany pradawnym i potężnym wzorcem Ondřeja.

### Z PHM instalujesz **tylko to, czego potrzebujesz**:

```bash
phm install php8.5-cli
phm install php8.5-fpm
phm install php8.5-redis
```

Nic więcej.  
Nic mniej.

Każdy pakiet jest:

- skompilowany wcześniej
- specyficzny dla architektury
- zgodny z ABI
- zamknięty w swoich zależnościach

Bez Homebrew.  
Bez lokalnej kompilacji.  
Bez ukrytej magii.

---

## 🧱 Czym Jest PHM

- **Binarnym menedżerem pakietów** dla PHP
- Narzędziem instalującym **gotowe komponenty PHP**
- Systemem, który respektuje:
  - architekturę CPU
  - stabilność ABI
  - deterministyczne buildy
- Sposobem na zakończenie rekompilowania PHP na każdej maszynie świata

---

## 🚫 Czym PHM Nie Jest

- ❌ nie jest phpenv  
- ❌ nie jest asdf  
- ❌ nie jest formułami brew  
- ❌ nie jest „po prostu zbuduj to lokalnie”  

PHM **nie udaje**, że kompilacja jest darmowa.  
PHM **nie udaje**, że programiści mają nieskończony czas.  
PHM **nie udaje**, że CO₂ jest wyimaginowane.

---

## 🔥 Prawdziwy Koszt Buildów

Każda niepotrzebna kompilacja kosztuje:

- energię elektryczną
- chłodzenie
- żywotność CPU
- skupienie programisty
- zasoby planety

Pomnóż to przez:

- pipeline’y CI
- laptopy
- zespoły
- firmy
- lata

> *Koszt jest realny — nawet jeśli terminal milczy.*

PHM istnieje, aby **zakończyć marnotrawstwo**.

---

## 🏗 Droga Naprzód

PHM buduje PHP **raz**.

A następnie dystrybuuje:

- `phpX.Y-cli`
- `phpX.Y-fpm`
- `phpX.Y-common`
- `phpX.Y-redis`
- `phpX.Y-amqp`
- `phpX.Y-mongodb`
- …

Każdy jako osobny artefakt.  
Każdy instalowalny niezależnie.  
Każdy powtarzalny.

To nie jest innowacja.

To **przypomnienie pradawnego, właściwego wzorca**.

---

## 🜂 Słowo Na Koniec

> *Potęga nie rodzi się z nieskończonej rekompilacji.*  
> *Potęga rodzi się z umiaru.*  
> *Z precyzji.*  
> *Z jednorazowego builda — i mądrego dzielenia się nim.*

PHM kroczy starą ścieżką.

A kuźnie wreszcie mogą ostygnąć.

---

## Dostępne Pakiety

### Pakiety Podstawowe (na wersję PHP)

| Pakiet | Opis |
|--------|------|
| `php8.5-common` | Współdzielone pliki, php.ini |
| `php8.5-cli` | Interpreter linii poleceń |
| `php8.5-fpm` | FastCGI Process Manager |
| `php8.5-cgi` | Binarka CGI |

### Rozszerzenia

| Pakiet | Opis |
|--------|------|
| `php8.5-opcache` | OPcache |
| `php8.5-redis` | Klient Redis |
| `php8.5-igbinary` | Serializator binarny |
| `php8.5-mongodb` | Sterownik MongoDB |
| `php8.5-amqp` | Klient RabbitMQ |
| `php8.5-xdebug` | Debugger |
| `php8.5-pcov` | Pokrycie kodu |
| `php8.5-memcached` | Klient Memcached |

---

## Linki

- **PHM CLI**: https://github.com/phm-dev/phm
- **Pakiety PHP**: https://github.com/phm-dev/php-packages

---

🜃 **PHM — Pakuj PHP raz. Instaluj wszędzie.**
Zainspirowany wzorcem **Ondřeja Surý'ego**.
Wykuwany dla współczesnych mrocznych krain.
