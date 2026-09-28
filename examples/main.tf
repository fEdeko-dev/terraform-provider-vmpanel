# Пример: машина Ubuntu 24.04 и вебхук биллинга.
#   export VMPANEL_ENDPOINT=https://panel.example.ru
#   export VMPANEL_API_KEY=vtk_…      # «Настройки» → «Ключи API»
terraform {
  required_providers {
    vmpanel = {
      source = "fedeko-dev/vmpanel"
    }
  }
}

provider "vmpanel" {}

data "vmpanel_os" "ubuntu" {
  name = "Ubuntu 24.04"
}

data "vmpanel_location" "main" {
  name = "Москва"
}

variable "root_password" {
  type      = string
  sensitive = true
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

resource "vmpanel_webhook" "billing" {
  url    = "https://billing.example.ru/panel-hook"
  events = ["vm.ready", "vm.build_failed", "vm.deleted"]
}

output "web_ip" {
  value = vmpanel_vm.web.ipv4
}
