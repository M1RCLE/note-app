// Frontend обращается к backend по относительному пути /api/...
// Запросы проксирует nginx (см. nginx.conf), поэтому CORS не нужен.
const list = document.getElementById("notes");
const form = document.getElementById("note-form");
const input = document.getElementById("note-text");
const statusEl = document.getElementById("status");

function setStatus(text) {
  statusEl.textContent = text;
}

function render(notes) {
  list.replaceChildren();
  for (const note of notes) {
    const li = document.createElement("li");

    const text = document.createElement("span");
    text.textContent = note.text; // textContent, а не innerHTML: защита от XSS

    const date = document.createElement("time");
    date.textContent = new Date(note.created_at).toLocaleString("ru-RU");

    const del = document.createElement("button");
    del.textContent = "✕";
    del.title = "Удалить";
    del.addEventListener("click", () => removeNote(note.id));

    li.append(text, date, del);
    list.append(li);
  }
  if (notes.length === 0) setStatus("Заметок пока нет");
}

async function loadNotes() {
  try {
    const resp = await fetch("/api/notes");
    if (!resp.ok) throw new Error(resp.status);
    setStatus("");
    render(await resp.json());
  } catch (e) {
    setStatus("Backend недоступен: " + e.message);
  }
}

async function removeNote(id) {
  await fetch(`/api/notes/${id}`, { method: "DELETE" });
  loadNotes();
}

form.addEventListener("submit", async (event) => {
  event.preventDefault();
  const text = input.value.trim();
  if (!text) return;
  const resp = await fetch("/api/notes", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ text }),
  });
  if (resp.ok) {
    input.value = "";
    loadNotes();
  } else {
    setStatus("Ошибка: " + resp.status);
  }
});

loadNotes();
