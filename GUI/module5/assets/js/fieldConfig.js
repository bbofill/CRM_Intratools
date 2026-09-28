//////////////////////////////////////////////////////////////////////////////////////////////////
//   Simplified field configuration for Module 5: Incomplete Users                              //
//   Only includes fields used in handleIncompleteUsers()                                       //
//////////////////////////////////////////////////////////////////////////////////////////////////

// Relation between DB field names and front-end option lists
export const formSources = {
  gender: "gender",
  birth_country: "countries",
  birth_province: "provinces",
  birth_city: "cities",
  nationality_code: "nationalities",
  academic_grade: "grades",
  certificat_I3: "certificat",
  vinculation_type: "vinculation",
  contracting_institution: ["institution", "universities"],
  job_category: "jobCategories",
  funding: "subvencions",

  graduation_university: "universities",
  graduation_country: "countries",
  grade_master_doctorate: "degreeType",
};

// Field configuration and display metadata
export const userFieldSections = {
  People: [
    { label: "Name", key: "people_name" },
    { label: "Surname", key: "surname" },
    { label: "Second surname", key: "secondSurname" },
    { label: "Gender", key: "gender", type: "select" },
    { label: "Birth date", key: "birth_date", type: "date" },
    { label: "Birth country", key: "birth_country", type: "select" },
    { label: "Birth province", key: "birth_province", type: "select" },
    { label: "Birth city", key: "birth_city", type: "select" },
    { label: "Nationality", key: "nationality_code", type: "select" },
    { label: "NIF", key: "nif" },
    { label: "Original NIF", key: "nif_extended" },
    { label: "Academic grade", key: "academic_grade", type: "select" },
    { label: "ORCID", key: "orcid" },
    { label: "Has I3 certificate?", key: "certificat_I3", type: "select" },
    { label: "Agrees to UNEIX?", key: "agreesToUneix", type: "boolean" },
    { label: "Is active?", key: "active", type: "boolean" }
  ],

  Contract: [
    { label: "Vinculation type", key: "vinculation_type", type: "select" },
    { label: "Contract start date", key: "start_date", type: "date" },
    { label: "Contract end date", key: "end_date", type: "date" },
    { label: "Job category", key: "job_category", type: "select" },
    { label: "Personal type", key: "type" },
    { label: "Weekly dedication hours", key: "totalDedication_hours" },
    { label: "Contracting institution", key: "contracting_institution", type: "select" },
    { label: "Funding", key: "funding", type: "select" }
  ],

  Education: [
    { label: "Degree type", key: "grade_master_doctorate", type: "select" },
    { label: "Graduation university", key: "graduation_university", type: "select" },
    { label: "Graduation country", key: "graduation_country", type: "select" },
    { label: "Graduation year", key: "graduation_year" }
  ]
};

export function getFieldConfig(field) {

  // casos manuals (visitants)

  if (field === "people_grade[degree].graduation_university") {
    return {
      key: "graduation_university",
      label: "Graduation university (Degree)",
      section: "Education",
      manual: true
    };
  }

  if (field === "people_grade[degree].graduation_year") {
    return {
      key: "graduation_year",
      label: "Graduation year (Degree)",
      section: "Education",
      manual: true
    };
  }
  if (field === "people_grade[degree].graduation_country") {
    return {
      key: "graduation_country",
      label: "Graduation country (Degree)",
      section: "Education",
      manual: true
    };
  }

  if (field === "people_grade[doctorate].graduation_university") {
    return {
      key: "graduation_university",
      label: "Graduation university (Doctorate)",
      section: "Education",
      manual: true
    };
  }

  if (field === "people_grade[doctorate].graduation_year") {
    return {
      key: "graduation_year",
      label: "Graduation year (Doctorate)",
      section: "Education",
      manual: true
    };
  }
  if (field === "people_grade[doctorate].graduation_country") {
    return {
      key: "graduation_country",
      label: "Graduation country (Doctorate)",
      section: "Education",
      manual: true
    };
  }

  const match = field.match(/^(\w+)(?:\[\d+\])?\.(.+)$/);
  if (!match) {
    console.warn("No match for regex →", field);
    console.groupEnd();
    return null;
  }

  const [, rawPrefix, keyRaw] = match;
  const sectionPrefix = rawPrefix.replace(/\[\d+\]/, "");
  const key = keyRaw.replace(/^people_grade\./, "");

  let section = null;
  switch (sectionPrefix) {
    case "people":
    case "people_nationality":
      section = userFieldSections.People;
      break;

    case "contract":
      section = userFieldSections.Contract;
      break;

    case "education":
    case "people_grade":
      section = userFieldSections.Education;
      break;

    default:
      section = Object.values(userFieldSections).flat();
  }

  const found = section.find(f => f.key === key);

  console.groupEnd();
  return found || null;
}
