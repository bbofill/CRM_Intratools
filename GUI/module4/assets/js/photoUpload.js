//////////////////////////////////////////////////////////////////////////////////////////////////
//                      Allows and formats profile picture modification                         //
//////////////////////////////////////////////////////////////////////////////////////////////////

//import Cropper from "/assets/js/cropper.esm.js";
import Cropper from "https://cdn.jsdelivr.net/npm/cropperjs@1.5.13/dist/cropper.esm.js";

export function setupPhotoUpload() {
  const input = document.getElementById("upload-photo");
  const img = document.getElementById("profile-img");
  const cropImg = document.getElementById("crop-image");
  const cropModalEl = document.getElementById("cropModal");

  if (!input || !img || !cropImg || !cropModalEl) return; // No inicializar si faltan elementos

  const cropModal = new bootstrap.Modal(cropModalEl);
  let cropper = null;

  input.addEventListener("change", () => {
    const file = input.files[0];
    if (!file) return;

    const url = URL.createObjectURL(file);

    const tempImage = new Image();
    tempImage.onload = () => {
      const { width, height } = tempImage;

      // Si la imagen no es cuadrada, abrir Cropper
      if (Math.abs(width - height) > 30) {
        cropImg.onload = () => {
          if (cropper) cropper.destroy();
          cropper = new Cropper(cropImg, {
            aspectRatio: 1,
            viewMode: 1,
            autoCropArea: 1,
            responsive: true,
            background: false,
            zoomable: true,
            movable: true,
            scalable: true,
            rotatable: false,
            minContainerWidth: 400,
            minContainerHeight: 400,
            minCropBoxWidth: 200,
            minCropBoxHeight: 200
          });
          cropModal.show();
        };
        cropImg.src = url;
      } else {
        img.src = url; // Imagen casi cuadrada → se usa tal cual
      }
    };
    tempImage.src = url;
  });

  document.getElementById("confirm-crop").addEventListener("click", () => {
    cropper.getCroppedCanvas({ width: 400, height: 400 }).toBlob(blob => {
      const newFile = new File([blob], "profile_picture.jpg", { type: "image/jpeg" });
      const dataTransfer = new DataTransfer();
      dataTransfer.items.add(newFile);
      document.getElementById("upload-photo").files = dataTransfer.files;
      document.getElementById("profile-img").src = URL.createObjectURL(newFile);
      bootstrap.Modal.getInstance(document.getElementById("cropModal")).hide();
    });
  });

}
