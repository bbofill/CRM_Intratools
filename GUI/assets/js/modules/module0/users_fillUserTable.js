// load user table from module backend and print
export async function loadUsersTable(endpoint = "/module/proxy/module0/api/users") {
  const tbody = document.getElementById('users-table-body');
  if (!tbody) return;

  tbody.innerHTML = `<tr><td colspan="6" class="text-center text-muted">Loading Data...</td></tr>`;

  try {
    const res = await fetch(endpoint);
    if (!res.ok) throw new Error("Error with network or backend");

    const users = await res.json();
    if (!Array.isArray(users)) throw new Error("Unexpected Answer");

    tbody.innerHTML = '';

    if (users.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="text-center text-muted">No users</td></tr>';
      return;
    }

    const roleOptions = ['admin', 'manager', 'support', 'user', 'guest'];

    for (const u of users) {
      const row = `
        <tr>
          <td>${u.people_id}</td>
          <td>${u.name}</td>
          <td>${u.surname}</td>
          <td>${u.username}</td>
          <td>
            <select data-user-id="${u.people_id}" class="role-select form-select form-select-sm">
              ${roleOptions.map(r => `<option value="${r}" ${r === u.role ? 'selected' : ''}>${r}</option>`).join('')}
            </select>
          </td>
          <td>
            <button class="btn btn-sm btn-select" onclick="modifyPasswordUser(${u.people_id}, '${u.username}')">✏️</button>
            <button class="btn btn-sm btn-danger" onclick="deleteUser(${u.people_id})">╳</button>
          </td>
        </tr>`;
      tbody.insertAdjacentHTML('beforeend', row);

      const lastSelect = tbody.querySelector('tr:last-child .role-select');
      lastSelect.addEventListener('change', async (e) => {
        const newRole = e.target.value;
        const peopleId = e.target.getAttribute('data-user-id');

        const payload = {
          people_id: peopleId,
          role: newRole
        };

        try {
          const res = await fetch("/module/proxy/module0/api/users?action=updateUsers", {
            method: "POST",
            headers: {
              "Content-Type": "application/json"
            },
            body: JSON.stringify(payload)
          });

          if (!res.ok) throw new Error("Failed to update role");
          console.log(`Role updated to ${newRole} for user ${peopleId}`);
        } catch (err) {
          console.error("Error updating role:", err);
          alert("Failed to update role");
        }
      });
    }

  } catch (err) {
    console.error("Error loading users:", err);
    tbody.innerHTML = `<tr><td colspan="6" class="text-danger text-center">Error loading data...</td></tr>`;
  }
}

// delete button
window.deleteUser = async function(id) {
  if (!confirm("Are you sure you want to delete this user?")) return;

  const payload = {
    method: "DELETE",
    table: "users",
    people_id: id
  };

  try {
    const res = await fetch("/module/proxy/module0/api/users", {
      method: "DELETE",
      headers: {
        "Content-Type": "application/json"
      },
      body: JSON.stringify(payload)
    });

    if (!res.ok) throw new Error("Error deleting user");
    await loadUsersTable();

  } catch (err) {
    alert("Failed to delete user.");
    console.error(err);
  }
};

// modify password 
window.modifyPasswordUser = function (id, username) {

  document.getElementById("changePasswordModalTitle").innerText =
    `Change password for user "${username}"`;
  const modal = new bootstrap.Modal(document.getElementById("changePasswordModal"));
  modal.show();

  // Remove previous listener to avoid duplicates
  const form = document.getElementById("changePasswordForm");
  const newForm = form.cloneNode(true);
  form.parentNode.replaceChild(newForm, form);

  newForm.addEventListener("submit", async (e) => {
    e.preventDefault();

    const newPass = document.getElementById("newPassword").value.trim();
    const newPass2 = document.getElementById("confirmNewPassword").value.trim();

    if (!newPass || !newPass2) {
      return alert("Please fill in both fields.");
    }

    if (newPass !== newPass2) {
      return alert("Password confirmation does not match.");
    }

    if (!/^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{6,}$/.test(newPass)) {
      return alert("Password must have at least 6 characters, one uppercase, one lowercase and one number.");
    }

    try {
      const hashed = await hashString(newPass);

      const res = await fetch("/module/proxy/module0/api/users?action=changePasswordAdmin", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          people_id: id,
          newPassword: hashed
        })
      });

      if (!res.ok) throw new Error("Failed to change password");

      alert("Password updated successfully.");
      modal.hide();

    } catch (err) {
      console.error(err);
      alert("Error updating password.");
    }
  });
};

document.querySelectorAll('.role-select').forEach(select => {
  select.addEventListener('change', async (e) => {
    const newRole = e.target.value;
    const peopleId = e.target.getAttribute('data-user-id');

    const payload = {
      people_id: peopleId,
      role: newRole
    };

    try {
      const res = await fetch("/module/proxy/module0/api/users", {
        method: "POST",
        headers: {
          "Content-Type": "application/json"
        },
        body: JSON.stringify(payload)
      });

      if (!res.ok) throw new Error("Failed to update role");
      console.log(`Role updated to ${newRole} for user ${peopleId}`);
    } catch (err) {
      console.error("Error updating role:", err);
      alert("Failed to update role");
    }
  });
});



// auxiliar function to hash passwords in SHA256
export async function hashString(text) {
  const encoder = new TextEncoder();
  const data = encoder.encode(text);
  const hashBuffer = await crypto.subtle.digest('SHA-256', data);
  return Array.from(new Uint8Array(hashBuffer))
    .map(b => b.toString(16).padStart(2, '0'))
    .join('');
}
