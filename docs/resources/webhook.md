---
page_title: "vmpanel_webhook — вебхук"
subcategory: ""
description: |-
  Вебхук: панель сообщает о событиях POST-запросом с подписью.
---

# vmpanel_webhook

Вебхук: панель сама сообщает о событиях POST-запросом с подписью
`X-Webhook-Signature: t=<unix>,v1=<hex>` — HMAC-SHA256 ключом `secret` от
строки `<t>.<тело>`. Не 2xx — повтор через 1, 5, 30 минут, 2, 6 и 12 часов.

## Пример

```terraform
resource "vmpanel_webhook" "billing" {
  url    = "https://billing.example.ru/panel-hook"
  events = ["vm.ready", "vm.build_failed", "vm.deleted"]
}
```

## Аргументы

- `url` (обязательно) — адрес получателя: только `https://` и только внешний.
- `events` (обязательно) — события: `vm.ready`, `vm.build_failed`, `vm.deleted`, `vm.mail_blocked`, `health.issue_opened`, `health.issue_resolved`; `["*"]` — все.
- `description` — заметка.
- `active` — включён ли (по умолчанию `true`).

## Атрибуты

- `id` — номер вебхука.
- `secret` (секрет) — ключ подписи. Панель отдаёт его только при создании,
  поэтому он есть лишь у вебхуков, созданных этим провайдером.

## Импорт

```shell
terraform import vmpanel_webhook.billing <id>
```
