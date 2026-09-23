/*
When user clicks submit, it checks:
1- All fields have content
2- Both current passwords match
3- New password fills all requierements
Then, sends old and new hashed passwords to backend
*/
export function setupPasswordChange() {
  document.getElementById("change-password-btn").addEventListener("click", () => {
    new bootstrap.Modal(document.getElementById("changePasswordModal")).show();
  });

  document.getElementById("changePasswordForm").addEventListener("submit", async function (e) {
    e.preventDefault();


    // Checks all fields are filled and both new passwords are the same
    const curr = document.getElementById("currentPassword").value.trim();
    const newPass = document.getElementById("newPassword").value.trim();
    const newPass2 = document.getElementById("confirmNewPassword").value.trim();

    if (!curr || !newPass2 || !newPass) return alert("Please fill in all fields.");
    if (newPass !== newPass2) return alert("New password confirmation does not match.");
    if (!/^(?=.*[a-z])(?=.*[A-Z])(?=.*\d).{6,}$/.test(newPass)) {
      return alert("Password must have at least 6 characters, one uppercase, one lowercase and one number.");
    }

    // sends hashed passwords to backend
    const res = await fetch("/module/proxy/module3/api?action=changePassword", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        oldPassword: await hashString(curr),
        newPassword: await hashString(newPass)
      })
    });

    if (res.ok) {
      alert("Password successfully updated.");
      bootstrap.Modal.getInstance(document.getElementById("changePasswordModal")).hide();
      e.target.reset();
    } else {
      alert("Error: Incorrect current password or server error.");
    }
  });
}


// auxiliar function to hash passwords in SHA256
export async function hashString(text) {
  const encoder = new TextEncoder();
  const data = encoder.encode(text);
  const hashBuffer = await crypto.subtle.digest('SHA-256', data);
  return Array.from(new Uint8Array(hashBuffer))
    .map(b => b.toString(16).padStart(2, '0'))
    .join('');
}
