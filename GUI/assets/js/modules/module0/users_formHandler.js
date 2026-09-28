export function initUserFormHandler(endpoint = "", reloadFn = null) {
    const form = document.getElementById("user-form");
    if (!form) return;
  
    form.addEventListener("submit", async function (e) {
      e.preventDefault();
  
      //const name = document.getElementById('name').value.trim();
      //const surnames = document.getElementById('surnames').value.trim();
      const username = document.getElementById('username').value.trim();
      const role = document.getElementById('role').value;
      const password = document.getElementById('password').value;
  
      const payload = {
        //name,
        //surnames,
        username,
        role,
        password
      };
  
      try {
        const res = await fetch(endpoint, {
          method: "POST",
          headers: {
            "Content-Type": "application/json"
          },
          body: JSON.stringify(payload)
        });
  
        if (!res.ok) {
          alert("❌ Error adding user");
          return;
        }
  
        form.reset();
        if (typeof reloadFn === "function") reloadFn();
  
      } catch (err) {
        console.error("Error sending form:", err);
        alert("Network Error");
      }
    });
  }
  
  