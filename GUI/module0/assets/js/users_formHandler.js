export async function initUserFormHandler(
  endpoint = "/module/proxy/module0/api/users",
  reloadFn = null
) {
  const form = document.getElementById("user-form");
  const workerSelect = document.getElementById("worker");
  const usernameInput = document.getElementById("username");
  const passwordInput = document.getElementById("password");

  // ---- Load list of available people into TomSelect ----
  async function loadAvailablePeople() {
    try {
      const res = await fetch("/module/proxy/module0/api/users?action=available-people");
      const people = await res.json();

      console.log("Available people:", people);

      new TomSelect(workerSelect, {
        maxItems: 1,
        placeholder: "Select worker...",
        valueField: "id",
        labelField: "text",
        searchField: "text",
        sortField: "text",
        options: people.map(p => ({
          id: p.id,
          text: [p.name, p.surname, p.secondSurname].filter(Boolean).join(" "),
          name: p.name,
          surname: p.surname
        })),
        render: {
          option: (item) => `<div>${item.text}</div>`,
          item: (item) => `<div>${item.text}</div>`
        },
        onChange(value) {
          const selected = people.find(p => p.id == value);
          if (!selected) return;

          fetch(`/module/proxy/module0/api/users?action=generate-username&name=${encodeURIComponent(selected.name)}&surname=${encodeURIComponent(selected.surname)}`)
            .then(res => res.json())
            .then(data => {
              usernameInput.value = data.username;
              passwordInput.value = data.password;
            });
        }
      });

    } catch (err) {
      console.error("Error loading available people:", err);
    }
  }

  await loadAvailablePeople();


  // ---- FORM SUBMIT ----
  if (!form) return;

  form.addEventListener("submit", async function (e) {
    e.preventDefault();

    const peopleId = workerSelect.tomselect.getValue();
    const username = usernameInput.value.trim();
    const role = document.getElementById("role").value;
    const password = passwordInput.value;

    if (!peopleId) {
      alert("Please select a worker.");
      return;
    }

    const payload = {
      people_id: Number(peopleId),
      username,
      role,
      password
    };

    try {
      const res = await fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });

      if (!res.ok) {
        const text = await res.text();
        alert("Error adding user: " + text);
        return;
      }

      alert("User created successfully");
      window.location.reload();

    } catch (err) {
      console.error("Error sending form:", err);
      alert("Network error");
    }
  });
}
