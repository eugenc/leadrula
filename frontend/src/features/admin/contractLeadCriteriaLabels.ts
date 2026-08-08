export function fieldsTabLabel(leadType: string, contractType: string): string {
  if (contractType === "buy") return "Required Fields";
  if (leadType === "Call") return "Call Fields";
  if (leadType === "Data" || leadType === "Appointment") return "Data Fields";
  return "Available Fields";
}

export function filtersTabLabel(_leadType: string): string {
  return "Match Criteria";
}

export function fieldsSectionCopy(leadType: string, contractType: string) {
  const title = fieldsTabLabel(leadType, contractType);
  if (contractType === "buy") {
    return {
      title,
      intro: "Required fields, mapping, and intake filters for leads on this contract.",
      addButton: "Add required field",
      removeAria: "Remove required field",
    };
  }
  if (leadType === "Call") {
    return {
      title,
      intro: "Fields accepted on call leads for this contract.",
      addButton: "Add call field",
      removeAria: "Remove call field",
    };
  }
  if (leadType === "Data" || leadType === "Appointment") {
    return {
      title,
      intro: "Fields accepted on leads for this contract.",
      addButton: "Add data field",
      removeAria: "Remove data field",
    };
  }
  return {
    title,
    intro: "Available fields, mapping, and intake filters for leads on this contract.",
    addButton: "Add available field",
    removeAria: "Remove available field",
  };
}

export function filtersSectionCopy() {
  return {
    title: "Match Criteria",
    intro: "Rules that must pass for a lead to match this contract.",
    addButton: "Add match rule",
    removeAria: "Remove match rule",
  };
}
