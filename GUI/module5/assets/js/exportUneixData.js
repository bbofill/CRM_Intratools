//////////////////////////////////////////////////////////////////////////////////////////////////
//             Formats every database field to the correct UNEIX option                         //
//                                and generates csv                                             //
//////////////////////////////////////////////////////////////////////////////////////////////////


// exportUneixData.js
export async function exportUneixCSV(completeWorkers, managerId = 98, directorId = 158, year) {
  if (!completeWorkers || completeWorkers.length === 0) {
    alert("No complete workers to export!");
    return;
  }
  const shortYear = String(year).slice(-2); 
  const filename = `RM${shortYear}46.csv`;

  let csvLines = [];

  completeWorkers.forEach(person => {
    const {
        general = {},
        contracts = [],
        grades = [],
        groups = [],
        nationalities = [],
        people_name = "",
        surname = "",
        secondSurname = "",
        visit_id = ""
    } = person;

    const nationality = nationalities[0]?.nationality_code || "";
    const birthDate = general.birth_date ? new Date(general.birth_date) : null;
    const birthYear = birthDate ? birthDate.getFullYear() : "";

    // calcula UNA VEZ el inicio más antiguo entre todos los contratos de la persona
    let minStartDate = null;

    contracts.forEach(c => {
      if (!c.start_date) return;
      const date = new Date(c.start_date);
      if (!isNaN(date)) {
        if (!minStartDate || date < minStartDate) {
          minStartDate = date;
        }
      }
    });
    
    const startYear = minStartDate ? String(minStartDate.getFullYear()) : "";

    // si vinculation_type=affiliated, contracting_institution=icrea, en UNEIX es vinculación contratado, contracting_institution CRM y subvencion ICREA
    contracts.forEach(contract => {

      if (contract.contracting_institution === "0000000469")
      {
        contract.vinculation_type = "Contracted worker"
        contract.job_category = "Researcher"
        contract.contracting_institution = "0000001672"
      }
      const startDateStr = contract.start_date
        ? contract.start_date.slice(0, 10).replaceAll("-", "")
        : "";

      const endDateStr = contract.end_date
        ? contract.end_date.slice(0, 10).replaceAll("-", "")
        : "99991231";
      const totalHours = contract.totalDedication_hours || "0000";
      
      let visitor = ""
      if (visit_id != "") visitor = "S";
      else visitor = (contract.vinculation_type || "").toLowerCase().includes("visitor") ? "S" : "N";
      
      const categoria = getCategoriaCode(person, contract, grades, visitor, general.academic_grade, year);

      const areaCode = getAreaCode(contract.vinculation_type, contract.job_category, categoria, groups);

      const vinc = getVincCode(contract.vinculation_type);

      let incorporationDate = String(general.first_incorporation_year);
      const incorporationYear = incorporationDate.slice(0,4);


      let carrec = ""
      // Si esta persona es el gerente → categoría 06
      if (Number(person.people_id) === Number(managerId)) {
        carrec = "06";
      } else if (Number(person.people_id) === Number(directorId)) {
        carrec = "13";
      } else {
        carrec = "00"
      }
      const {
        universityCode,
        universityCountry,
        graduationYear,
        phdUniversityCode,
        phdUniversityCountry,
        phdGraduationYear
      } = getLatestDegreeInfo(grades, year);

      let researchHours;
      let gestionHours;
      if (categoria === "46") {
        researchHours = "0000";
        gestionHours = "0000";
      } else if (vinc != "B" || visitor === "S") {
        researchHours = ["21","26","22","23","25","70", "24"].includes(categoria) ? totalHours : "0000";
        gestionHours = ["46","45"].includes(categoria) ? totalHours : "0000";
      } else {
        gestionHours = "0000";
        researchHours = totalHours;
      }
      const codiGrup = getGroupCode(person, grades, year);




      const line = [
        year, "0000001672", "0000001672", areaCode,
        general.nif_extended, general.nif,
        sanitizeForUneix(people_name), sanitizeForUneix(surname), sanitizeForUneix(secondSurname || ""),
        birthYear, general.birth_city || "",
        (general.birth_province || ""),
        (general.birth_country || ""),
        nationality, general.gender, general.academic_grade,
        incorporationYear, vinc, categoria, carrec,
        "0000", totalHours, contract.funding || "",
        "SA", universityCode || "", universityCountry || "", graduationYear || "",phdUniversityCode || "", phdUniversityCountry || "", phdGraduationYear || "", "2", "0000",
        researchHours, gestionHours, visitor, "", codiGrup,
        startDateStr, endDateStr, mapUniversityCodeContractant(contract.contracting_institution) || "",
        general.orcid || "", "", general.certificat_I3 == 1 ? "S" : "N"
      ].join("|");

      csvLines.push(line);
    });
  });

  // Crear y descargar el CSV
  const blob = new Blob([csvLines.join("\n")], { type: "text/csv;charset=utf-8;" });
  const link = document.createElement("a");
  link.href = URL.createObjectURL(blob);
  link.download = filename;
  link.click();
}
function getAreaCode(vinculationType, jobCategory, categoria, groups) {
  console.log("groups: ", groups)

  const vt = (vinculationType || "").toLowerCase();
  const jc = (jobCategory || "").toLowerCase().trim();

  if (categoria === "45" && jc === "ktu") return "999";

  // Si pertany a un grup, se li dona la categoria d'aquell grup
  if (groups.length != 0 && groups[0].group_type !== null) {
    console.log("group type: ", groups[0].group_type)
    return String(groups[0].group_type)
  }
  if (vt === "affiliated") return "595";
  if (vt === "visitor" || vt === "internship§ tfg or tfm") return "999";

  if (vt === "contracted worker") {
    const isResearchBucket =
      jc.includes("researcher") ||
      jc.includes("research technician") ||
      jc.includes("phd");
    return isResearchBucket ? "595" : "999";
  }
  return "999";
}
function getVincCode(vinculationType) {

  if (!vinculationType) return "";
  const vt = vinculationType.toLowerCase().trim();
  if (vt === "contracted worker") return "4";
  if (vt === "affiliated" || vt === "visitor" || vt === "b") return "B";
  if (vt === "internship§ tfg or tfm") return "A";

  return "";
}

function getCategoriaCode(person, contract, grades, visitor, status, currentYear) {
  const vinc = (contract.vinculation_type || "").toLowerCase().trim();
  const job = (contract.job_category || "").toLowerCase().trim();
  const ctype = (contract.type || "").toLowerCase().trim();

  console.log(status)
  // Internship, TFG o TFM = 70
  if (vinc === "internship§ tfg or tfm" || (visitor === "S" && status === "5") || (visitor === "S" && status === "6")) return "70";

  // IT o persona con ID 160 = 46
  if (job === "it-maintenance" || person.people_id === 160) return "46";

  console.log(vinc)
  // Researcher con doctorado = 21 / 26 / 22
  if (job === "researcher" || job === "postdoc" || (vinc === "b" && status == 1) || (vinc === "affiliated" && status == 1)) {
    console.log("entra")
    const docGrade = grades.find(
      g => (g.grade_master_doctorate || "").toLowerCase() === "doctorate" && g.graduation_year
    );
    console.log(docGrade)
    if (docGrade) {
      const diff = currentYear - parseInt(docGrade.graduation_year);
      if (diff > 5) 
      {
        console.log("21"); 
        return "21";
      }
      if (diff > 2) 
        {
          console.log("26"); 
          return "26";
        }
      console.log("22")
      return "22";
    }
  }

  // PhD = 23
  if (job === "phd" || (visitor === "S" && status === "7")) return "23";

  // KTU con tipo "direct structural support" = 45
  if (job === "ktu" && ctype === "direct structural support") return "45";

  // KTU (otros) = 24
  if (job === "ktu") return "24";

  // Research technician = 25
  if (job === "research technician") return "25";


  // Default = 45
  return "45";
}


function getLatestDegreeInfo(grades, year) {
  if (!grades || grades.length === 0) {
    return {
      universityCode: "", universityCountry: "", graduationYear: "",
      phdUniversityCode: "", phdUniversityCountry: "", phdGraduationYear: ""
    };
  }

  // Filtrar grados que tengan tipo válido y año de graduación
  const validDegrees = grades.filter(g =>
    g.graduation_year && g.graduation_year <= year &&
    ["bachelor¤s degree", "master¤s degree"].includes((g.grade_master_doctorate || "").toLowerCase())
  );

  // Filtrar doctorados (por separado)
  const validPhDs = grades.filter(g =>
    g.graduation_year && g.graduation_year <= year &&
    (g.grade_master_doctorate || "").toLowerCase() === "doctorate"
  );

  // ---- Grado o máster más reciente ----
  let latestDegree = { graduation_university: "", graduation_country: "", graduation_year: "" };
  if (validDegrees.length > 0) {
    validDegrees.sort((a, b) => parseInt(b.graduation_year) - parseInt(a.graduation_year));
    latestDegree = validDegrees[0];
  }

  // ---- Doctorado más reciente ----
  let latestPhD = { graduation_university: "", graduation_country: "", graduation_year: "" };
  if (validPhDs.length > 0) {
    validPhDs.sort((a, b) => parseInt(b.graduation_year) - parseInt(a.graduation_year));
    latestPhD = validPhDs[0];
  }

  // Campos base
  let universityCode = mapUniversityCode(latestDegree.graduation_university);
  let universityCountry = latestDegree.graduation_country || "";
  let graduationYear = latestDegree.graduation_year || "";

  // Campos doctorado
  let phdUniversityCode = mapUniversityCode(latestPhD.graduation_university);
  let phdUniversityCountry = latestPhD.graduation_country || "";
  let phdGraduationYear = latestPhD.graduation_year || "";

  return {
    universityCode,
    universityCountry,
    graduationYear,
    phdUniversityCode,
    phdUniversityCountry,
    phdGraduationYear
  };
}

function getGroupCode(person, grades, currentYear) {
  const academicGrade = (person.general?.academic_grade || "").toString().trim();

  // Si es Doctor (grade == 1) y hay doctorado
  if (academicGrade === "1") {
    // Buscar el doctorado más antiguo
    const phdGrades = grades.filter(
      g => (g.grade_master_doctorate || "").toLowerCase() === "doctorate" && g.graduation_year
    );

    if (phdGrades.length > 0) {
      phdGrades.sort((a, b) => parseInt(a.graduation_year) - parseInt(b.graduation_year)); // más antiguo primero
      const oldestPhD = phdGrades[0];
      const diff = currentYear - parseInt(oldestPhD.graduation_year);

      if (diff >= 25) return "D";
      if (diff >= 20) return "C";
      if (diff >= 15) return "B";
      if (diff >= 10) return "A";
    }
  }

  // Si no cumple lo anterior = resto de casos
  switch (academicGrade) {
    case "1":
    case "7":
    case "2":
    case "6":
      return "1";
    case "3":
      return "2";
    case "4":
      return "3";
    case "5":
      return "4";
    case "G":
      return "5";
    default:
      return "6"; // sin grupo
  }
}


function mapUniversityCode(rawValue) {
  if (!rawValue) return "";

  const value = rawValue.trim().toUpperCase();

  // Si ya está en las equivalencias manuales
  if (SPANISH_EQUIVALENCES[value]) {
    return SPANISH_EQUIVALENCES[value];
  }


  // Si empieza por prefijos extranjeros = Extranjera
  const foreignPrefixes = ["D ", "DZA", "DEU", "ARG", "AND", "AFG", "ALB", "ATG", "AGO", "SAU"];
  if (foreignPrefixes.some(prefix => value.startsWith(prefix))) {
    return "99";
  }

  // Si empieza por "E " (universidad/institución española no listada)
  if (value.startsWith("E ")) {
    return "98";
  }

  // Si no se encuentra nada = por defecto, “otras españolas”
  return "99";
}

function mapUniversityCodeContractant(rawValue) {
  if (!rawValue) return "";

  const value = rawValue.trim().toUpperCase();

  // Si ya está en las equivalencias manuales
  if (SPANISH_EQUIVALENCES[value]) {
    return SPANISH_EQUIVALENCES[value];
  } else {
    return rawValue
  }
}

function sanitizeForUneix(value) {
  if (value === null || value === undefined) return "";

  // Aseguramos que sea string
  let str = String(value);

   const replacements = {
    "¤": "'",
  };

  str = str.replace(/./g, ch => (replacements[ch] !== undefined ? replacements[ch] : ch));

  //    á -> a, ñ -> n, ç -> c, ü -> u, etc.
  str = str.normalize("NFD").replace(/[\u0300-\u036f]/g, "");

  str = str.replace(/[\u0000-\u001F\u007F]/g, "");

  return str;
}

export const SPANISH_EQUIVALENCES = {
  "E  ALCAL-H01": "29",
  "E  ALICANT01": "01",
  "E  ALMERIA01": "48",
  "E  AVILA01": "59",
  "E  BADAJOZ01": "02",
  "E  BARCELO01": "04",
  "E  BARCELO02": "22",
  "E  BARCELO03": "24",
  "E  BARCELO11": "98",
  "E  BARCELO15": "39",
  "E  BARCELO16": "41",
  "E  BARCELO21": "54",
  "E  BARCELO22": "98",
  "E  BARCELO24": "62",
  "E  BARCELO25": "98",
  "E  BARCELO28": "98",
  "E  BARCELO29": "98",
  "E  BARCELO91": "98",
  "E  BARCELO92": "98",
  "E  BARCELO93": "98",
  "E  BARCELO94": "98",
  "E  BARCELO95": "98",
  "E  BARCELO96": "98",
  "E  BARCELO97": "98",
  "E  BARCELO98": "98",
  "E  BARCELO99": "70",
  "E  BILBAO01": "20",
  "E  BILBAO02": "98",
  "E  BILBAO06": "98",
  "E  BURGOS01": "51",
  "E  BURGOS02": "98",
  "E  BURGOS90": "98",
  "E  CADIZ01": "05",
  "E  CASTELL01": "40",
  "E  CAT-EMPRESA": "98",
  "E  CIUDA-R01": "98",
  "E  CORDOBA01": "06",
  "E  CORDOBA03": "98",
  "E  ELCHE01": "55",
  "E  EMPRESA": "98",
  "E  GIRONA01": "98",
  "E  GIRONA02": "43",
  "E  GRANADA01": "08",
  "E  GRANADA04": "98",
  "E  HUELVA01": "49",
  "E  JAEN01": "50",
  "E  LA-CORU01": "98",
  "E  LAS-PAL01": "26",
  "E  LAS-PAL04": "98",
  "E  LEON01": "09",
  "E  LLEIDA01": "44",
  "E  LOGRONO01": "45",
  "E  LOGRONO16": "98",
  "E  MAD-EMPRESA": "98",
  "E  MADRID01": "28",
  "E  MADRID02": "98",
  "E  MADRID03": "10",
  "E  MADRID04": "23",
  "E  MADRID05": "25",
  "E  MADRID07": "98",
  "E  MADRID08": "98",
  "E  MADRID09": "98",
  "E  MADRID12": "52",
  "E  MADRID14": "98",
  "E  MADRID17": "47",
  "E  MADRID18": "53",
  "E  MADRID19": "98",
  "E  MADRID21": "46",
  "E  MADRID22": "98",
  "E  MADRID25": "98",
  "E  MADRID26": "56",
  "E  MADRID27": "98",
  "E  MADRID28": "68",
  "E  MADRID32": "98",
  "E  MADRID33": "65",
  "E  MADRID98": "98",
  "E  MADRID99": "68",
  "E  MALAGA01": "11",
  "E  MANRESA": "60",
  "E  MATARO01": "98",
  "E  MONCADA01": "71",
  "E  MONDRAG01": "61",
  "E  MURCIA01": "12",
  "E  MURCIA02": "98",
  "E  MURCIA03": "98",
  "E  MURCIA04": "64",
  "E  MURCIA05": "66",
  "E  NAV-EMPRESA": "98",
  "E  OVIEDO01": "13",
  "E  PALMA01": "03",
  "E  PALMA02": "98",
  "E  PAMPLON01": "98",
  "E  PAMPLON02": "98",
  "E  SALAMAN01": "98",
  "E  SALAMAN02": "14",
  "E  SALAMAN03": "98",
  "E  SAN-SEB01": "98",
  "E  SANTAND01": "16",
  "E  SANTIAG01": "07",
  "E  SEGOVIA01": "57",
  "E  SEVILLA01": "17",
  "E  SEVILLA02": "63",
  "E  SEVILLA03": "58",
  "E  TARRAGO01": "42",
  "E  TENERIF01": "15",
  "E  VALENCI01": "18",
  "E  VALENCI02": "27",
  "E  VALENCI06": "98",
  "E  VALENCI07": "98",
  "E  VALENCI08": "71",
  "E  VALENCI09": "98",
  "E  VALENCI11": "72",
  "E  VALLADO01": "19",
  "E  VALLADO02": "98",
  "E  VALLADO99": "69",
  "E  VIC01": "60",
  "E  VIGO01": "98",
  "E  VITORIA01": "98",
  "E  VITORIA03": "98",
  "E  VITORIA98": "98",
  "E  ZARAGOZ01": "21",
  "E  ZARAGOZ04": "98",
  "E  ZARAGOZ07": "73",
  "E ALBACET01": "98",
  "E CORDOBA23": "98",
  "E SANTANDER99": "67",
  "ESP999": "98"
};
