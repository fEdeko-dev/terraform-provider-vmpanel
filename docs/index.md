---
page_title: "Провайдер vmpanel"
description: |-
  Виртуальные машины и вебхуки панели управления хостинга как код.
---

# Провайдер vmpanel

Виртуальные машины, вебхуки и справочники панели управления хостинга на
Proxmox VE как код. Провайдер работает через API панели с ключом API, поэтому
подходит всем:

- **клиенту хостинга** — ключ видит только его машины;
- **сотруднику хостинга** — весь парк его организации;
- **владельцу платформы** — всю платформу.

Ключ выпускается в панели: «Настройки» → «Ключи API». Клиенту хостинга, у
которого нет доступа к панели, ключ выдаёт личный кабинет хостинга
(`POST /vm/v3/api_key`).

## Пример

```terraform
terraform {
  required_providers {
    vmpanel = {
      source = "fedeko-dev/vmpanel"
    }
  }
}

provider "vmpanel" {
  endpoint = "https://panel.example.ru"
  # api_key — лучше через переменную VMPANEL_API_KEY
}

data "vmpanel_os" "ubuntu" {
  name = "Ubuntu 24.04"
}

resource "vmpanel_vm" "web" {
  name     = "web-1"
  os_id    = data.vmpanel_os.ubuntu.id
  cpu      = 2
  ram_mib  = 2048
  disk_mib = 20480
  password = var.root_password
}
```

## Настройки провайдера

- `endpoint` (строка) — адрес панели. Или переменная `VMPANEL_ENDPOINT`.
- `api_key` (строка, секрет) — ключ API `vtk_…`. Или переменная `VMPANEL_API_KEY`.
