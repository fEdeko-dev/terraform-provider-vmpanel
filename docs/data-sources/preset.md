---
page_title: "vmpanel_preset — тариф"
subcategory: ""
description: |-
  Поиск: тариф по названию.
---

# vmpanel_preset

Поиск по названию без учёта регистра — тариф. Не нашлось — ошибка со
списком того, что есть.

## Пример

```terraform
data "vmpanel_preset" "this" {
  name = "VPS-2"
}
```

## Атрибуты

- `id` — номер для ресурсов провайдера.
- `cpu`, `ram_mib`, `disk_mib` — ресурсы тарифа.
