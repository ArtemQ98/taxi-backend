# KEY v0.8 — Booking & Meetings

KEY — Marketplace + Fleet OS для аренды автомобилей.

## Что в v0.8

- календарь выбора диапазона дат в стиле travel-booking;
- занятые даты конкретного автомобиля отображаются серыми и недоступны;
- серверная проверка пересечений броней защищает от двойного бронирования;
- новая бронь получает статус `pending` и ждёт подтверждения владельца;
- оплата только офлайн: владелец вручную отмечает оплату полученной;
- владелец назначает встречу для получения и возврата;
- клиент видит назначенные дату, время и место встречи в «Моих поездках»;
- фотографии автомобилей сохраняются и показываются в Marketplace;
- никаких демо-пользователей, машин, автопарков и аренд.

## Запуск

```bash
docker compose down

docker compose up --build
```

Для полностью чистой базы один раз:

```bash
docker compose down -v
docker compose up --build
```

## Адреса

- Marketplace: http://localhost/
- Кабинет владельца: http://localhost/app
- API health: http://localhost/api/health

## Логика брони

```text
клиент выбирает даты
        ↓
свободные / занятые даты
        ↓
pending
        ↓
владелец подтверждает
        ↓
назначает получение
        ↓
владелец вручную подтверждает оплату
        ↓
выдача
        ↓
активная аренда
        ↓
возврат
        ↓
закрытие
```

Оплата на сайте намеренно не проводится: KEY фиксирует только факт, который подтверждает владелец.


## KEY Taxi

The project now includes a lightweight taxi marketplace on the same KEY visual system.

- Client: no registration; search by city/settlement and call an available driver.
- Driver: register/login, fill in name, phone, city, car brand/model and plate.
- Driver status: `Свободен` / `Занят`.
- Public API: `GET /api/public/taxi-drivers?city=...`
- Driver API: `GET/PATCH /api/taxi/profile`, `PATCH /api/taxi/status`.
- Database migration: `server/migrations/013_taxi.sql`.

The existing KEY backend and infrastructure remain in place; the taxi UI is the default client experience.
