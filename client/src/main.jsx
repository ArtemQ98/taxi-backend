import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import {
  ArrowUpRight,
  Bell,
  CarFront,
  Check,
  ChevronRight,
  CircleDot,
  LogOut,
  Menu,
  Phone,
  Search,
  ShieldCheck,
  UserRound,
  X,
} from "lucide-react";
import "./styles.css";

const API = import.meta.env.VITE_API_URL || "http://localhost:8080/api";

async function api(path, options = {}) {
  const token = localStorage.getItem("key_token");
  const headers = {
    "Content-Type": "application/json",
    ...(options.headers || {}),
  };
  if (token) headers.Authorization = `Bearer ${token}`;
  const res = await fetch(`${API}${path}`, { ...options, headers });
  let data = null;
  try {
    data = await res.json();
  } catch {}
  if (!res.ok) throw new Error(data?.error || "Не удалось выполнить запрос");
  return data;
}

function PrivacyPage({ onBack }) {
  return (
    <div className="privacy-page">
      <header className="privacy-topbar">
        <button className="back-link" onClick={onBack}>
          ← Назад
        </button>
        <Logo />
      </header>
      <main className="privacy-content">
        <span className="kicker">Документы</span>
        <h1>Политика конфиденциальности</h1>
        <p className="muted">Дата последнего обновления: 17.09.2026</p>

        <h3>1. Общие положения</h3>
        <p>
          1.1. Настоящая Политика определяет порядок обработки персональных
          данных и меры по обеспечению их безопасности, применяемые
          оператором — самозанятым гражданином, зарегистрированным в
          соответствии с Федеральным законом № 422-ФЗ (далее — Оператор).
        </p>
        <p>
          1.2. Оператор: самозанятый. Контактный e-mail:{" "}
          <a href="mailto:ArtTeam71@yandex.ru">ArtTeam71@yandex.ru</a>.
          Регион деятельности: Тульская область.
        </p>
        <p>
          1.3. Политика разработана в соответствии с Федеральным законом
          от 27.07.2006 № 152-ФЗ «О персональных данных».
        </p>

        <h3>2. Категории обрабатываемых данных</h3>
        <p>2.1. Оператор обрабатывает следующие персональные данные
          пользователей-водителей: имя; номер телефона; город / населённый
          пункт; сведения об автомобиле (марка, модель, государственный номер).
        </p>
        <p>
          2.2. Для пользователей-клиентов (поиск такси) персональные данные не
          собираются. Обрабатываются только технические данные (IP-адрес,
          время запроса) в логах сервера.
        </p>

        <h3>3. Цели обработки</h3>
        <p>3.1. Данные водителей обрабатываются исключительно для:
          предоставления возможности клиентам найти свободного водителя;
          отображения контактной информации водителя в результатах поиска;
          связи клиента с водителем.
        </p>
        <p>
          3.2. Данные не используются для рекламных рассылок, маркетинга или
          передачи третьим лицам в иных целях.
        </p>

        <h3>4. Правовые основания</h3>
        <p>4.1. Обработка осуществляется на основании: согласия субъекта
          (водителя), выраженного при регистрации; необходимости исполнения
          договора, стороной которого является субъект (п. 5 ч. 1 ст. 6 152-ФЗ).
        </p>

        <h3>5. Порядок и сроки обработки</h3>
        <p>5.1. Обработка включает: сбор, запись, хранение, использование,
          отображение в поиске.
        </p>
        <p>
          5.2. Данные хранятся до отзыва согласия субъектом или до удаления
          аккаунта. После этого данные удаляются в течение 30 дней.
        </p>

        <h3>6. Права субъекта</h3>
        <p>6.1. Субъект вправе: запросить информацию об обработке своих
          данных; потребовать уточнения, блокирования или удаления данных;
          отозвать согласие, направив запрос на{" "}
          <a href="mailto:ArtTeam71@yandex.ru">ArtTeam71@yandex.ru</a>.
        </p>
        <p>6.2. Запрос обрабатывается в течение 30 дней.</p>

        <h3>7. Меры безопасности</h3>
        <p>7.1. Оператор принимает необходимые правовые, организационные и
          технические меры для защиты данных от неправомерного доступа:
          передача данных по HTTPS; хранение в защищённой базе данных;
          ограничение доступа к серверной инфраструктуре.
        </p>

        <h3>8. Заключительные положения</h3>
        <p>8.1. Оператор вправе вносить изменения в настоящую Политику.
          Актуальная версия всегда доступна по адресу{" "}
          <a href="/privacy">/privacy</a>.
        </p>
        <p>
          8.2. Контакт для обращений:{" "}
          <a href="mailto:ArtTeam71@yandex.ru">ArtTeam71@yandex.ru</a>.
        </p>
      </main>
    </div>
  );
}

function Logo() {
  return (
    <div className="logo">
      <span>TAXI</span>
      <i />
    </div>
  );
}

function Auth({ onDone, onBack }) {
  const [mode, setMode] = useState("login");
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [name, setName] = useState("");
  const [phone, setPhone] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  const [consent, setConsent] = useState(false);
  const [showPrivacy, setShowPrivacy] = useState(false);

  async function submit(e) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      const body =
        mode === "login"
          ? { identifier, password }
          : { name, phone, email: "", password, company_name: "KEY Taxi" };
      const data = await api(
        mode === "login" ? "/auth/login" : "/auth/register",
        {
          method: "POST",
          body: JSON.stringify(body),
        },
      );
      localStorage.setItem("key_token", data.token);
      onDone(data.user);
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <main className="auth">
      <section className="auth-side">
        <Logo />
        <div className="auth-copy">
          <span className="kicker">Операционная система такси</span>
          <h1>
            Свободный водитель — <em>в один клик.</em>
          </h1>
          <p>
            Профиль водителя, статус, город и автомобиль — в одном рабочем
            пространстве KEY.
          </p>
          <div className="proof-row">
            <span>
              <Check size={15} /> без регистрации клиента
            </span>
            <span>
              <Check size={15} /> живой статус
            </span>
            <span>
              <Check size={15} /> прямой звонок
            </span>
          </div>
        </div>
        <div className="auth-footer-note">KEY · taxi OS</div>
      </section>
      <section className="auth-panel">
        <div className="auth-panel-inner">
          <button className="back-link" onClick={onBack}>
            ← К поиску
          </button>
          <span className="kicker">
            {mode === "login" ? "Вход водителя" : "Новый водитель"}
          </span>
          <h2>
            {mode === "login" ? "С возвращением." : "Запустим ваш профиль."}
          </h2>
          <p className="muted">
            {mode === "login"
              ? "Войдите по телефону или email."
              : "Создайте аккаунт, затем добавьте автомобиль и город."}
          </p>
          <form onSubmit={submit} className="form">
            {mode === "register" && (
              <>
                <Field label="Имя">
                  <input
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder="Иван"
                    required
                  />
                </Field>
                <Field label="Телефон">
                  <input
                    value={phone}
                    onChange={(e) => setPhone(e.target.value)}
                    placeholder="+7 999 123-45-67"
                    inputMode="tel"
                    autoComplete="tel"
                    required
                  />
                </Field>
              </>
            )}
            {mode === "login" && (
              <Field label="Телефон или email">
                <input
                  value={identifier}
                  onChange={(e) => setIdentifier(e.target.value)}
                  placeholder="+7 999 123-45-67"
                  inputMode="tel"
                  autoComplete="tel"
                  required
                  autoFocus
                />
              </Field>
            )}
            <Field label="Пароль">
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                placeholder="Минимум 8 символов"
                required
                minLength={8}
              />
            </Field>
            {error && (
              <div className="form-error">
                <X size={16} />
                {error}
              </div>
            )}
            {mode === "register" && (
              <label className="consent">
                <input
                  type="checkbox"
                  checked={consent}
                  onChange={(e) => setConsent(e.target.checked)}
                />
                <span>
                  Я согласен на обработку персональных данных и принимаю{" "}
                  <button
                    type="button"
                    className="link-inline"
                    onClick={() => setShowPrivacy(true)}
                  >
                    Политику конфиденциальности
                  </button>
                </span>
              </label>
            )}
            <button
              className="btn primary wide"
              disabled={loading || (mode === "register" && !consent)}
            >
              {loading
                ? "Подождите…"
                : mode === "login"
                  ? "Войти в KEY"
                  : "Создать профиль"}
              <ArrowUpRight size={18} />
            </button>
          </form>
          <button
            className="text-link"
            onClick={() => {
              setMode(mode === "login" ? "register" : "login");
              setError("");
            }}
          >
            {mode === "login" ? "Создать аккаунт" : "Уже есть аккаунт → войти"}
          </button>
        </div>
      </section>
      {showPrivacy && (
        <div className="modal-overlay" onClick={() => setShowPrivacy(false)}>
          <div className="modal" onClick={(e) => e.stopPropagation()}>
            <PrivacyPage onBack={() => setShowPrivacy(false)} />
          </div>
        </div>
      )}
    </main>
  );
}

function Field({ label, children }) {
  return (
    <label className="field">
      <span>{label}</span>
      {children}
    </label>
  );
}

function SearchPage({ onDriver }) {
  const [city, setCity] = useState("");
  const [drivers, setDrivers] = useState([]);
  const [loading, setLoading] = useState(false);
  const [searched, setSearched] = useState(false);
  const [error, setError] = useState("");

  async function search(e) {
    e?.preventDefault();
    setLoading(true);
    setError("");
    setSearched(true);
    try {
      const data = await api(
        `/public/taxi-drivers?city=${encodeURIComponent(city.trim())}`,
      );
      setDrivers(data);
    } catch (err) {
      setError(err.message);
      setDrivers([]);
    } finally {
      setLoading(false);
    }
  }

  return (
    <div className="public-shell">
      <header className="public-topbar">
        <Logo />
        <div className="public-actions">
          <span className="online-note">
            <i /> Сервис работает
          </span>
          <button className="btn secondary" onClick={onDriver}>
            <UserRound size={16} /> Водителям
          </button>
        </div>
      </header>

      <main className="public-page">
        <section className="hero">
          <span className="kicker">KEY · TAXI</span>
          <h1>
            Найти свободное <em>такси.</em>
          </h1>
          <p>Выберите посёлок или город. Регистрация клиента не нужна.</p>
          <form className="search-card" onSubmit={search}>
            <div className="search-icon">
              <Search size={20} />
            </div>
            <div className="search-field">
              <span>Где нужен водитель</span>
              <input
                value={city}
                onChange={(e) => setCity(e.target.value)}
                placeholder="Тула"
                autoComplete="address-level2"
                autoFocus
              />
            </div>
            <button className="btn primary" disabled={loading || !city.trim()}>
              {loading ? "Ищем…" : "Найти такси"}
              <ArrowUpRight size={18} />
            </button>
          </form>
        </section>

        {error && <div className="notice error">{error}</div>}

        {searched && (
          <section className="results">
            <div className="section-head">
              <div>
                <span className="kicker">Результаты поиска</span>
                <h2>Свободные такси в {city}</h2>
              </div>
              <span className="count">{drivers.length}</span>
            </div>
            {drivers.length === 0 && !loading ? (
              <div className="empty card">
                <CarFront size={28} />
                <b>Свободных водителей пока нет</b>
                <span>Попробуйте другой город или посёлок.</span>
              </div>
            ) : (
              <div className="driver-grid">
                {drivers.map((d) => (
                  <DriverCard key={d.id} driver={d} />
                ))}
              </div>
            )}
          </section>
        )}

        {!searched && (
          <section className="how-grid">
            <Info title="1. Введите город" text="Например, Тула." />
            <Info
              title="2. Выберите водителя"
              text="В поиске отображаются только свободные."
            />
            <Info
              title="3. Позвоните"
              text="Контакт водителя открывается прямо в карточке."
            />
          </section>
        )}
      </main>
    </div>
  );
}

function Info({ title, text }) {
  return (
    <div className="info-card card">
      <span className="step">01</span>
      <b>{title}</b>
      <p>{text}</p>
    </div>
  );
}

function DriverCard({ driver: d }) {
  return (
    <article className="driver-card card">
      <div className="driver-top">
        <div className="person-avatar">{(d.name || "В").slice(0, 1)}</div>
        <div>
          <span className="available">
            <i /> Свободен
          </span>
          <h3>{d.name}</h3>
        </div>
      </div>
      <div className="car-block">
        <div className="car-icon">
          <CarFront size={22} />
        </div>
        <div>
          <b>
            {d.car_brand} {d.car_model}
          </b>
          <small>{d.plate}</small>
        </div>
      </div>
      <div className="driver-meta">
        <span>
          <CircleDot size={13} /> {d.city}
        </span>
        <span>KEY verified</span>
      </div>
      <a className="call-btn" href={`tel:${d.phone}`}>
        <Phone size={17} /> Позвонить <ArrowUpRight size={16} />
      </a>
    </article>
  );
}

function DriverDashboard({ user, onLogout, onBack }) {
  const [profile, setProfile] = useState(null);
  const [form, setForm] = useState({
    name: user?.name || "",
    phone: user?.phone || "",
    city: "",
    car_brand: "",
    car_model: "",
    plate: "",
  });
  const [loading, setLoading] = useState(true),
    [saving, setSaving] = useState(false),
    [error, setError] = useState("");

  async function load() {
    try {
      const p = await api("/taxi/profile");
      setProfile(p);
      setForm({
        name: p.name || "",
        phone: p.phone || "",
        city: p.city || "",
        car_brand: p.car_brand || "",
        car_model: p.car_model || "",
        plate: p.plate || "",
      });
    } catch (err) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }
  useEffect(() => {
    load();
  }, []);

  async function save(e) {
    e.preventDefault();
    setSaving(true);
    setError("");
    try {
      const p = await api("/taxi/profile", {
        method: "PATCH",
        body: JSON.stringify(form),
      });
      setProfile(p);
    } catch (err) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }
  async function toggle() {
    try {
      const p = await api("/taxi/status", {
        method: "PATCH",
        body: JSON.stringify({
          status: profile.status === "available" ? "busy" : "available",
        }),
      });
      setProfile(p);
    } catch (err) {
      setError(err.message);
    }
  }

  if (loading)
    return (
      <div className="loading-screen">
        <Logo />
        <span>Загружаем KEY…</span>
      </div>
    );

  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="side-top">
          <Logo />
        </div>
        <div className="workspace">
          <div className="workspace-avatar">
            {(form.name || "В").slice(0, 1)}
          </div>
          <div>
            <b>KEY Taxi</b>
            <small>{form.city || "Город не указан"} · водитель</small>
          </div>
          <ChevronRight size={15} />
        </div>
        <nav>
          <button className="active">
            <CarFront size={18} /> Профиль водителя
          </button>
          <button onClick={onBack}>
            <Search size={18} /> Поиск такси
          </button>
        </nav>
        <div className="side-bottom">
          <div className="security-card">
            <ShieldCheck size={17} />
            <div>
              <b>Контур безопасности</b>
              <span>
                Профиль доступен клиентам только в статусе «Свободен».
              </span>
            </div>
          </div>
          <button className="logout" onClick={onLogout}>
            <LogOut size={17} /> Выйти
          </button>
        </div>
      </aside>

      <section className="main">
        <header className="topbar">
          <div className="crumb">
            KEY <span>/</span> Профиль водителя
          </div>
          <div className="top-actions">
            {/* <div className="notif">
              <Bell size={18} />
            </div> */}
            <div className="top-avatar">{(form.name || "В").slice(0, 1)}</div>
          </div>
        </header>
        <main className="page">
          <div className="page-head">
            <div>
              <span className="kicker">Водитель</span>
              <h1>Ваш профиль.</h1>
              <p>Настройте данные и управляйте видимостью в поиске.</p>
            </div>
            <button
              className={`status-toggle ${profile?.status === "available" ? "on" : ""}`}
              onClick={toggle}
            >
              <i />
              {profile?.status === "available" ? "Свободен" : "Занят"}
              <span />
            </button>
          </div>

          {error && <div className="notice error">{error}</div>}

          <div className="dashboard-grid">
            <form className="card profile-form" onSubmit={save}>
              <div className="card-head">
                <div>
                  <span className="kicker">Данные</span>
                  <h3>Информация водителя</h3>
                </div>
              </div>
              <div className="form-grid">
                <Field label="Имя">
                  <input
                    value={form.name}
                    onChange={(e) => setForm({ ...form, name: e.target.value })}
                    required
                  />
                </Field>
                <Field label="Телефон">
                  <input
                    value={form.phone}
                    onChange={(e) =>
                      setForm({ ...form, phone: e.target.value })
                    }
                    required
                  />
                </Field>
                <Field label="Посёлок / город">
                  <input
                    value={form.city}
                    onChange={(e) => setForm({ ...form, city: e.target.value })}
                    placeholder="Тула"
                    required
                  />
                </Field>
                <Field label="Марка">
                  <input
                    value={form.car_brand}
                    onChange={(e) =>
                      setForm({ ...form, car_brand: e.target.value })
                    }
                    placeholder="Toyota"
                    required
                  />
                </Field>
                <Field label="Модель">
                  <input
                    value={form.car_model}
                    onChange={(e) =>
                      setForm({ ...form, car_model: e.target.value })
                    }
                    placeholder="Camry"
                    required
                  />
                </Field>
                <Field label="Госномер">
                  <input
                    value={form.plate}
                    onChange={(e) =>
                      setForm({ ...form, plate: e.target.value.toUpperCase() })
                    }
                    placeholder="А123ВС 777"
                    autoCapitalize="characters"
                    autoComplete="off"
                    required
                  />
                </Field>
              </div>
              <button className="btn primary" disabled={saving}>
                {saving ? "Сохраняем…" : "Сохранить профиль"}
                <ArrowUpRight size={18} />
              </button>
            </form>

            <div className="card live-card">
              <div className="card-head">
                <div>
                  <span className="kicker">Live</span>
                  <h3>Как вас видит клиент</h3>
                </div>
                <span className="count">PREVIEW</span>
              </div>
              <DriverCard
                driver={{ ...form, id: "preview", status: profile?.status }}
              />
            </div>
          </div>
        </main>
        {/* Мобильная навигация */}
        <nav className="mobile-bottom-nav" aria-label="Навигация">
          <button type="button" onClick={onBack}>
            <Search size={20} />
            <span>Поиск</span>
          </button>

          <button type="button" className="active" aria-current="page">
            <UserRound size={20} />
            <span>Профиль</span>
          </button>

          <button type="button" onClick={onLogout} className="logout-mobile">
            <LogOut size={20} />
            <span>Выйти</span>
          </button>
        </nav>
      </section>
    </div>
  );
}

function App() {
  const [view, setView] = useState("search");
  const [user, setUser] = useState(null);

  useEffect(() => {
    if (localStorage.getItem("key_token")) {
      api("/me")
        .then((u) => {
          setUser(u);
          setView("driver");
        })
        .catch(() => localStorage.removeItem("key_token"));
    }
  }, []);

  function logout() {
    localStorage.removeItem("key_token");
    setUser(null);
    setView("search");
  }
  if (window.location.pathname === "/privacy") {
    return <PrivacyPage onBack={() => (window.location.href = "/")} />;
  }
  if (view === "auth")
    return (
      <Auth
        onDone={(u) => {
          setUser(u);
          setView("driver");
        }}
        onBack={() => setView("search")}
      />
    );
  if (view === "driver")
    return (
      <DriverDashboard
        user={user}
        onLogout={logout}
        onBack={() => setView("search")}
      />
    );
  return <SearchPage onDriver={() => setView(user ? "driver" : "auth")} />;
}

createRoot(document.getElementById("root")).render(<App />);
