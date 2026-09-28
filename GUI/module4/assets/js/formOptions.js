//////////////////////////////////////////////////////////////////////////////////////////////////
//             This file contains all the options for the select-type fields                    //
//////////////////////////////////////////////////////////////////////////////////////////////////


// Options defined manually
export const formOptions = {
  gender: [
    { "code": "D", "name": "Female" },
    { "code": "H", "name": "Male" }
  ],
  vinculation: [
    "Contracted worker",
    "Affiliated",
    "Visitor",
    "Internship, TFG or TFM"
  ],
  institution: [
    { "code": "0000001672", "name": "CRM: Centre de Recerca Matemàtica" }
  ],
  trainee: [
    "TFG",
    "TFM",
    "Curricular",
    "Extracurricular"
  ],
  visitor: [
    "PhD",
    "Postdoc",
    "Undergraduate",
    "Researcher"
  ],
  jobCategories: [
    "PhD",
    "Postdoc",
    "Researcher",
    "Open Science",
    "Research Technician",
    "Activities",
    "Communication",
    "Management",
    "RRHH",
    "Administration",
    "Finances",
    "Projects",
    "KTU",
    "IT-Maintenance",
    "Secretary"
  ],
  department: [
    "Structural research",
    "Conjunctural research",
    "Direct conjunctural support",
    "Direct structural support",
    "Indirect structural support",
    "Indirect conjunctural support"
  ],
  degreeType: [
    "Bachelor's degree",
    "Master's degree",
    "Doctorate"
  ],
  responsible: [
    "IP",
    "Tutor"
  ],
  uniAffiliated: [
    {
      "code": "0000001672",
      "name": "CRM: Centre de Recerca Matemàtica"
    },
    {
      "code": "E  BARCELO02",
      "name": "UNIVERSITAT AUTÒNOMA DE BARCELONA"
    },
    {
      "code": "E  BARCELO03",
      "name": "UNIVERSITAT POLITÈCNICA DE CATALUNYA"
    },
    {
      "code": "E  BARCELO01",
      "name": "UNIVERSITAT DE BARCELONA"
    },
    {
      "code": "0000000469",
      "name": "ICREA"
    }
  ],
  trainingCategories: [
    "PRL",
    "Languages",
    "Technical training",
    "Equality/diversity training",
    "Complementary academic traning",
    "Professional certifications"
  ],
  recognition: [
    { code: "001", name: "Generalitat Recognition" },
    { code: "002", name: "Center Recognition" },
    { code: "005", name: "TECNIO d'Acció Recognition" },
    { code: "003", name: "Other recognitions" },
  ],
  character: [
    { code: "1", name: "Public" },
    { code: "2", name: "Private" },
    { code: "4", name: "Mixed" },
  ],
  typology: [
    { code: "6", name: "Transfer Unit" },
    { code: "7", name: "Facilities or Platforms Unit" },
    { code: "5", name: "Other" },
  ]
};


// Options defined from the db tables
let countryOptions = [];

export async function fetchOptions() {
  const response = await fetch('/module/proxy/module4/api?action=get-options');
  const data = await response.json();

  Object.keys(formOptions).forEach(key => {
    if (Array.isArray(formOptions[key])) {
      formOptions[key] = formOptions[key].map(opt =>
        typeof opt === "string" ? { code: opt, name: opt } : opt
      );
    }
  });

  countryOptions = data.country.map(u => ({
    code: u.code,
    name: u.name_EN,
    nationality: u.name_nationality
  }));

  formOptions.countries = countryOptions.map(c => ({
    code: c.code,
    name: c.name
  }));

  formOptions.nationalities = countryOptions.map(c => ({
    code: c.code,
    name: c.nationality
  }));
  formOptions.provinces = data.provinces.map(p => ({
    code: p.code,
    name: p.name
  }));

  formOptions.cities = data.cities.map(c => ({
    code: c.code,
    name: c.name,
    province: c.provinceCode
  }))
  formOptions.grades = data.educationGrade.map(g => ({
    code: g.code,
    name: g.name
  }));
  formOptions.universities = data.universities.map(u => ({
    code: u.code,
    name: u.name
  }));
  formOptions.studies = data.studies.map(u => ({
    code: u.code,
    name: u.name
  }));
  formOptions.supervisor = data.people.map(u => ({
    code: u.id,
    name: `${u.name} ${u.surname}${u.secondSurname ? ' ' + u.secondSurname : ''}`
  }));
  formOptions.office = data.room.map(u => ({
    code: u.id,
    name: `${u.name} (${u.uab_code})`
  }));
  formOptions.desk = data.hottable.map(u => ({
    code: u.id,
    name: u.name
  }));
  formOptions.researchArea = data.researchArea.map(u => ({
    code: u.id,
    name: u.name
  }));
  formOptions.training = data.training.map(u => ({
    code: u.id,
    name: u.name
  }));
  formOptions.groups = data.researchGroup.map(u => ({
    code: u.intern_code,
    name: `${u.category} - ${u.name}`
  }));
  formOptions.knowledgeArea = data.knowledge_area.map(u => ({
    code: u.code,
    name: u.name
  }));
  formOptions.subvencions = data.fundings.map(u => ({
    code: u.code,
    name: u.name,
    funding_active: Number(u.active) === 1
  }));
}
