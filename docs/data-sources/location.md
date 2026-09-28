---
page_title: "vmpanel_location — локация"
subcategory: ""
description: |-
  Поиск: локация по названию.
---

# vmpanel_location

Поиск по названию без учёта регистра — локация. Не нашлось — ошибка со
списком того, что есть.

## Пример

```terraform
data "vmpanel_location" "this" {
  name = "Москва"
}
```

## Атрибуты

- `id` — номер для ресурсов провайдера.

