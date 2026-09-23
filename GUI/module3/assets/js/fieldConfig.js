//////////////////////////////////////////////////////////////////////////////////////////////////
//             This file contains all the relations of the field's names                        //
//                      database <-> front-end <-> form options                                 //
//////////////////////////////////////////////////////////////////////////////////////////////////


// Relation of the db field name with the form options shown at front-end
export const formSources = {
  gender: "gender",
  birth_country: "countries",
  birth_province: "provinces",
  birth_city: "cities",
  residence_country: "countries",
  residence_province: "provinces",
  residence_city: "cities",
  nationality_code: "nationalities",
  academic_grade: "grades",
  vinculation_type: "vinculation",
  contracting_institution: ["institution", "universities"],
  trainee_type: ["trainee", "visitor"],
  trainee_studies: "studies",
  job_category: "jobCategories",
  contract_type: "department",
  supervisor_id: "supervisor",
  office_location: "office",
  research_interests: "researchArea",
  funding: "subvencions",
  grade_master_doctorate: "degreeType",
  graduation_university: "universities",
  grade_code: "studies",
  graduation_country: "countries",
  phd_university: "universities",
  ip_or_tutor: "responsible",
  people_id: "supervisor",
  training_id: "training",
  group_intern_code: "groups"
};


// Key is the field name in database (with some exceptions to differenciate
// fields from different tables with the same name)
// Label is the tag shown at the front end
// Each field belongs to a section, moving them from section and/or order
// will reflect the change in the modal
export const userFieldSections = {
  General: [
    { label: "Name", key: "people_name" },
    { label: "Preferred name", key: "prefered_name" },
    { label: "Surname", key: "surname" },
    { label: "Second surname", key: "secondSurname" },
    { label: "Gender", key: "gender" },
    { label: "Birth date", key: "birth_date", type: "date" },
    { label: "Birth country", key: "birth_country" },
    { label: "Birth province (if birth country is Spain)", key: "birth_province" },
    { label: "Birth city (if birth province is in Catalunya)", key: "birth_city" },
    { label: "NIF", key: "nif" },
    { label: "Original NIF", key: "nif_extended" },
    { label: "User email", key: "user_email" },
    { label: "User phone", key: "user_phone" },
    { label: "Emergency contact name", key: "emergencyContact_name" },
    { label: "Emergency contact phone", key: "emergencyContact_phone" },
    { label: "CRM email", key: "crm_email" },
    { label: "Highest academic grade", key: "academic_grade" },
    { label: "Research interests", key: "research_interests" },
    { label: "ORCID", key: "orcid" },
    { label: "Has I3 certificate?", key: "certificat_I3", type: "boolean" },
    { label: "Personal web page", key: "personal_webPage" },
    { label: "Observations", key: "observations" },
    { label: "Residence", key: "residence", type: "repeater" },
    { label: "Nationality", key: "nationality", type: "repeater" }
  ],
  ResidenceItem: [
    { label: "Country", key: "residence_country" },
    { label: "Province (if country is Spain)", key: "residence_province" },
    { label: "City (if province is in Catalunya)", key: "residence_city" },
    { label: "Postal code", key: "postal_code" },
    { label: "Address", key: "address" },
    { label: "Is it the actual residence?", key: "actual", type: "boolean" }
  ],
  NationalityItem: [
    { label: "Nationality", key: "nationality_code" }
  ],
  Contract: [
    { label: "Contract", key: "contract", type: "repeater" }
  ],
  ContractItem: [
    { label: "Vinculation", key: "vinculation_type" },
    { label: "Contracting institution", key: "contracting_institution" },
    { label: "Type", key: "trainee_type" },
    { label: "Studies linked to training", key: "trainee_studies" },
    { label: "Has an internship?", key: "internship", type: "boolean" },
    { label: "Contract start date", key: "contract_start_date", type: "date" },
    { label: "Contract end date", key: "contract_end_date", type: "date" },
    { label: "Job category", key: "job_category" },
    { label: "Personal type", key: "contract_type" },
    { label: "Position", key: "position" },
    { label: "Dedication hours per week", key: "totalDedication_hours" },
    { label: "Supervisor", key: "supervisor", type: "repeater" },
    { label: "Office location", key: "office_location" },
    { label: "Funding", key: "funding" },
    { label: "Contract file", key: "contract_file_path", type: "file" }
  ],  
  SupervisorItem: [
    { label: "Supervisor", key: "supervisor_id" },
  ],
  Education: [
    { label: "Education", key: "education", type: "repeater" }
  ],
  EducationItem: [
    { label: "Type of degree", key: "grade_master_doctorate" },
    { label: "Area", key: "grade_code" },
    { label: "Studies", key: "gradeName" },
    { label: "Graduation university", key: "graduation_university" },
    { label: "If your university is 'Other', please specify its name", key: "grade_university_name" },
    { label: "University country", key: "graduation_country" },
    { label: "Graduation year", key: "graduation_year", type: "year" },
    { label: "Education", key: "education", type: "repeater" }
  ],
  Phd: [
    { label: "Phd", key: "phd", type: "repeater" }
  ],
  PhdItem: [
    { label: "Phd program", key: "phd_program" },
    { label: "Phd University", key: "phd_university" },
    { label: "Start year", key: "phd_startYear", type: "year" },
    { label: "Thesis director", key: "phd_tesisDirector" },
    { label: "Thesis title", key: "phd_tesisTitle" },
    { label: "Planned presentation date", key: "phd_plannedPresentationDate", type: "date" },
    { label: "Presentation date", key: "phd_presentationDate", type: "date" },
    { label: "TESEO link or similar", key: "phd_link" },
    { label: "Responsible", key: "responsible", type: "repeater" },

  ],
  ResponsibleItem: [
    { label: "Type of responsible", key: "ip_or_tutor" },
    { label: "Responsible, if available in CRM database", key: "people_id" },
    { label: "If the resposible is not listed, please specify their name", key: "name" },
  ],
  Training: [
    { label: "Training", key: "training", type: "repeater" }
  ],
  TrainingItem: [
    { label: "Course name", key: "training_id" },
    { label: "Completion date (if finished)", key: "training_date", type: "date" },
    { label: "Diploma file", key: "training_diploma_path", type: "file" }
  ],
  Groups: [
    { label: "Groups", key: "groups", type: "repeater" }
  ],
  GroupsItem: [
    { label: "Group name", key: "group_intern_code" },
    { label: "Start date in the group", key: "group_start_date", type: "date" },
    { label: "End date in the group", key: "group_end_date", type: "date" },
    { label: "Is the worker the group’s IP?", key: "group_ip", type: "boolean" }
  ],
  Projects: [
    { label: "Projects", key: "projects", type: "repeater" }
  ],
  ProjectsItem: [
    { label: "Project name", key: "project_name" },
    { label: "Project ID", key: "project_id" },
    { label: "Project number", key: "project_number" },
    { label: "Role", key: "researcher_type" },
    { label: "Start date in the project", key: "project_start_date", type: "date" },
    { label: "End date in the project", key: "project_end_date", type: "date" }
  ]
};
