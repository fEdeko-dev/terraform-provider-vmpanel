---
page_title: "vmpanel_vm — виртуальная машина"
subcategory: ""
description: |-
  Виртуальная машина: создание, смена ресурсов на месте, импорт.
---

# vmpanel_vm

Виртуальная машина. Создание ждёт, пока машина станет `active`: адрес и
пароль уже настроены в госте, можно сразу входить. Ядра, память, диск и
пароль меняются без пересоздания; имя, система, локация и тариф — через
пересоздание.

## Пример

```terraform
data "vmpanel_os" "ubuntu" {
  name = "Ubuntu 24.04"
}

data "vmpanel_location" "main" {
  name = "Москва"
}

resource "vmpanel_vm" "web" {
  name        = "web-1"
  os_id       = data.vmpanel_os.ubuntu.id
  location_id = data.vmpanel_location.main.id
  cpu         = 2
  ram_mib     = 2048
  disk_mib    = 20480
  password    = var.root_password
}

output "ip" {
  value = vmpanel_vm.web.ipv4
}
```

## Аргументы

- `name` (обязательно) — имя: латиница, цифры, `-`, `.`, до 49 символов. Смена — пересоздание.
- `os_id` (обязательно) — система, `data.vmpanel_os.<…>.id`. Смена — пересоздание.
- `password` (обязательно, секрет) — пароль root (Administrator у Windows), 6–128 печатных символов латиницы без пробела. Смена применяется в работающей машине.
- `location_id` — локация, `data.vmpanel_location.<…>.id`; не задана — первая доступная. Смена — пересоздание.
- `preset_id` — тариф, `data.vmpanel_preset.<…>.id`: задаёт ядра, память и диск, если они не указаны. Смена — пересоздание.
- `cpu` — ядра. Смена — перезапуском машины, панель делает его сама.
- `ram_mib` — память, МиБ. Смена — перезапуском машины.
- `disk_mib` — диск, МиБ. Только увеличение: раздел и файловая система растягиваются в госте.

## Атрибуты

- `id` — номер машины в API панели.
- `ipv4` — основной адрес; `ipv4_addresses` — все адреса IPv4.
- `state` — `active`, `stopped`, `suspended`, `creating`, `crashed`.
- `os_name` — установленная система; `node` — нода Proxmox.

## Импорт

```shell
terraform import vmpanel_vm.web 42
```

Номер — `id` машины в `/vm/v3/host`. Система восстанавливается по названию.
Пароль панель не хранит, поэтому первый `apply` после импорта установит
пароль из конфигурации.
